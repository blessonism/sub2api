package service

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const channelMonitorDisplayTTL = 30 * time.Second
const channelMonitorDisplayTimeout = 2 * time.Second

// ChannelMonitorMetricKey 的模型使用请求侧名称，与 V2 归一化口径一致。
type ChannelMonitorMetricKey struct {
	GroupID int64
	Model   string
}

type ChannelMonitorDisplayMetrics struct {
	CacheRate24h *float64
	OutputTPS5m  *float64
}

type ChannelMonitorDisplayRepository interface {
	BatchMetrics(context.Context, []ChannelMonitorMetricKey, time.Time) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error)
}

type channelMonitorDisplayGroups interface {
	GetAvailableGroups(context.Context, int64) ([]Group, error)
}

type channelMonitorDisplayPending struct {
	done    chan struct{}
	err     error
	metrics ChannelMonitorDisplayMetrics
}

type channelMonitorDisplayCacheEntry struct {
	metrics ChannelMonitorDisplayMetrics
	expires time.Time
}

// ChannelMonitorDisplayService 只缓存分组/模型统计；权限和个人可见倍率逐请求解析。
type ChannelMonitorDisplayService struct {
	repo    ChannelMonitorDisplayRepository
	groups  channelMonitorDisplayGroups
	now     func() time.Time
	mu      sync.Mutex
	cache   map[ChannelMonitorMetricKey]channelMonitorDisplayCacheEntry
	pending map[ChannelMonitorMetricKey]*channelMonitorDisplayPending
}

func NewChannelMonitorDisplayService(repo ChannelMonitorDisplayRepository, groups *APIKeyService) *ChannelMonitorDisplayService {
	return &ChannelMonitorDisplayService{repo: repo, groups: groups, now: time.Now,
		cache: make(map[ChannelMonitorMetricKey]channelMonitorDisplayCacheEntry)}
}

// Enrich 的可选展示查询失败时不影响主动探测结果。
func (s *ChannelMonitorDisplayService) Enrich(ctx context.Context, userID int64, views []*UserMonitorView) {
	for _, view := range views {
		view.VisibleRateMultiplier, view.CacheRate24h, view.OutputTPS5m = nil, nil, nil
	}
	if s == nil || s.groups == nil || userID <= 0 || len(views) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, channelMonitorDisplayTimeout)
	defer cancel()
	groups, err := s.groups.GetAvailableGroups(ctx, userID)
	if err != nil {
		slog.Warn("channel_monitor: display group lookup failed", "error", err)
		return
	}
	allowed := make(map[int64]float64, len(groups))
	for _, group := range groups {
		// GetAvailableGroups 已解析专属展示倍率和当前分时策略。
		allowed[group.ID] = group.RateMultiplier
	}
	targets := make(map[ChannelMonitorMetricKey]struct{})
	for _, view := range views {
		if view.GroupID == nil {
			continue
		}
		rate, ok := allowed[*view.GroupID]
		if !ok {
			continue
		}
		view.VisibleRateMultiplier = &rate
		if view.CheckMode != MonitorCheckModeQuota && view.PrimaryModel != "" && view.PrimaryModel != "quota" {
			targets[ChannelMonitorMetricKey{*view.GroupID, strings.TrimSpace(view.PrimaryModel)}] = struct{}{}
		}
	}
	if len(targets) == 0 || s.repo == nil {
		return
	}
	keys := make([]ChannelMonitorMetricKey, 0, len(targets))
	for key := range targets {
		keys = append(keys, key)
	}
	metrics, err := s.metrics(ctx, keys)
	if err != nil {
		slog.Warn("channel_monitor: display metrics unavailable", "error", err)
		return
	}
	for _, view := range views {
		if view.GroupID == nil || view.VisibleRateMultiplier == nil || view.CheckMode == MonitorCheckModeQuota {
			continue
		}
		m := metrics[ChannelMonitorMetricKey{*view.GroupID, strings.TrimSpace(view.PrimaryModel)}]
		view.CacheRate24h, view.OutputTPS5m = m.CacheRate24h, m.OutputTPS5m
	}
}

// metrics 对每个分组/模型领取查询所有权，重叠的批次共享在途结果。
// 同一次调用中未在途的 key 仍然合为一条批量 SQL。
func (s *ChannelMonitorDisplayService) metrics(ctx context.Context, keys []ChannelMonitorMetricKey) (map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, error) {
	now := s.now()
	result := make(map[ChannelMonitorMetricKey]ChannelMonitorDisplayMetrics, len(keys))
	waiting := make(map[ChannelMonitorMetricKey]*channelMonitorDisplayPending)
	owned := make([]ChannelMonitorMetricKey, 0, len(keys))
	s.mu.Lock()
	if s.pending == nil {
		s.pending = make(map[ChannelMonitorMetricKey]*channelMonitorDisplayPending)
	}
	for key, entry := range s.cache {
		if !entry.expires.After(now) {
			delete(s.cache, key)
		}
	}
	for _, key := range keys {
		if entry, ok := s.cache[key]; ok {
			result[key] = entry.metrics
			continue
		}
		pending, ok := s.pending[key]
		if !ok {
			pending = &channelMonitorDisplayPending{done: make(chan struct{})}
			s.pending[key] = pending
			owned = append(owned, key)
		}
		waiting[key] = pending
	}
	s.mu.Unlock()
	if len(owned) > 0 {
		// 首个 HTTP 请求取消不取消其他用户正在等待的统计；SQL 始终受 2 秒上限约束。
		go func() {
			queryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), channelMonitorDisplayTimeout)
			defer cancel()
			loaded, err := s.repo.BatchMetrics(queryCtx, owned, now.UTC())
			expires := s.now().Add(channelMonitorDisplayTTL)
			s.mu.Lock()
			defer s.mu.Unlock()
			for _, key := range owned {
				pending := s.pending[key]
				if err == nil {
					s.cache[key] = channelMonitorDisplayCacheEntry{loaded[key], expires}
				}
				pending.err = err
				pending.metrics = loaded[key]
				delete(s.pending, key)
				close(pending.done)
			}
		}()
	}
	for key, pending := range waiting {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-pending.done:
			if pending.err != nil {
				return nil, pending.err
			}
			result[key] = pending.metrics
		}
	}
	return result, nil
}
