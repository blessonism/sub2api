package service

import (
	"context"
	"encoding/json"
	"time"
)

const settingKeyModelIQProbeRunState = "model_iq_probe_run_state"

// 后台任务状态通过 settings 共享，不依赖浏览器连接或单个应用实例的内存。
type ModelIQProbeRunState struct {
	ID           string                   `json:"id"`
	Status       string                   `json:"status"`
	StartedAt    *time.Time               `json:"started_at"`
	FinishedAt   *time.Time               `json:"finished_at"`
	Summary      ModelIQProbeCycleSummary `json:"summary"`
	ErrorMessage string                   `json:"error_message,omitempty"`
}

func (s *ModelIQProbeService) GetRunState(ctx context.Context) (*ModelIQProbeRunState, error) {
	state := &ModelIQProbeRunState{Status: "idle"}
	if s.settings == nil {
		return state, nil
	}
	raw, err := s.settings.GetRawSettingValue(ctx, settingKeyModelIQProbeRunState)
	if ErrSettingNotFound.Is(err) {
		return state, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(raw), state); err != nil {
		return nil, err
	}
	// 进程异常退出无法写结束状态，超时后不再把旧任务显示为运行中。
	if state.Status == "running" && state.StartedAt != nil && time.Since(*state.StartedAt) > modelIQProbeRunTimeout+time.Minute {
		state.Status = "failed"
		state.ErrorMessage = "probe interrupted or timed out"
	}
	return state, nil
}

func (s *ModelIQProbeService) saveRunState(ctx context.Context, state ModelIQProbeRunState) error {
	if s.settings == nil {
		return ErrModelIQProbeUnavailable
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return s.settings.SetRawSettingValue(ctx, settingKeyModelIQProbeRunState, string(payload))
}

// 终态短暂写入失败时重试；整个过程受调用方期限约束，不延长互斥锁生存期。
func (s *ModelIQProbeService) saveFinalRunState(ctx context.Context, state ModelIQProbeRunState) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, time.Second)
		lastErr = s.saveRunState(attemptCtx, state)
		cancel()
		if lastErr == nil {
			return nil
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(attempt+1) * 100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return lastErr
}
