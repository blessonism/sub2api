// Package sessiongroup 把同一会话下的多次模型调用归并到一个稳定单元，
// 供观测层在 Langfuse 侧按会话聚合、按线程/步骤读出完整对话。
//
// 四条约束：
//   - 纯内存：只维护锚点 → 会话/线程/步骤的小表，不产生任何 I/O、不写日志、不落盘；
//   - 只是读取：不感知计费、不改变对外行为，也不回写任何业务字段；
//   - 客户端显式会话头优先，缺失时才用「用户身份 + 首条用户消息」推导；
//   - 有界：表大小超限先淘汰过期项，仍超限则整体重置，宁可再生步骤编号也不吃内存。
package sessiongroup

import (
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/prompttext"
)

const (
	// DefaultTTL 是锚点的空闲过期时间：超过它再出现即视为新会话
	// （推导 id 重新生成、步骤从 1 开始）。
	DefaultTTL = 30 * time.Minute

	// DefaultMaxEntries 是内存表上限。
	DefaultMaxEntries = 4096

	// attemptWindow 内出现同一请求体，视为「同一步的又一次尝试」（客户端重试/并行），
	// 而不是新的一步。
	attemptWindow = 2 * time.Minute

	// titleLimit 是会话标题的字节上限（按 rune 边界回退）。
	titleLimit = 180

	// fingerprintLimit 是参与指纹的消息字节上限：锚点消息本身已截断，
	// 这里再兜一层，避免超长单条消息把 hash 成本抬起来。
	fingerprintLimit = 2048
)

// 会话来源：客户端显式给出 / 系统推导 / 无法归组。
const (
	SourceClient = "client"
	SourceAuto   = "auto"
	SourceNone   = "none"
)

// Input 是一次调用可用于归组的全部事实。
type Input struct {
	// ClientSessionID 是客户端显式会话标识（session_id / X-Session-Id 等）。
	ClientSessionID string
	UserID          int64
	APIKeyID        int64
	// FirstUserPrompt 是请求里第一条用户消息（已跳过客户端注入的包装块）。
	FirstUserPrompt string
	// RequestFingerprint 是请求体指纹，用于识别「同一步的重复尝试」。
	RequestFingerprint string
	// Now 取本次请求的时间，缺失时用当前时间。
	Now time.Time
}

// Group 是一次调用的归组结果。
type Group struct {
	// SessionID 是会话聚合键，空表示无法归组（与历史行为一致：不产生 session）。
	SessionID string
	// SessionSource 取 client / auto / none。
	SessionSource string
	// ThreadID 是会话内的逻辑线程标识：同一会话里并行的多条对话流靠它区分。
	ThreadID string
	// StepIndex 是线程内第几次上游请求（从 1 开始）。
	StepIndex int
	// AttemptIndex 是同一步的第几次尝试（从 1 开始，>1 表示客户端重试/并行）。
	AttemptIndex int
	// Title 是会话标题，取锚点消息首行。
	Title string
}

type entry struct {
	sessionID string
	lastSeen  time.Time
	step      int
	attempt   int
	lastFP    string
	lastFPAt  time.Time
}

// Registry 是归组状态表。零值不可用，用 NewRegistry 构造。
type Registry struct {
	mu         sync.Mutex
	entries    map[string]*entry
	ttl        time.Duration
	maxEntries int
}

// NewRegistry 构造状态表。ttl / maxEntries 非正值时用默认值。
func NewRegistry(ttl time.Duration, maxEntries int) *Registry {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if maxEntries <= 0 {
		maxEntries = DefaultMaxEntries
	}
	return &Registry{entries: make(map[string]*entry), ttl: ttl, maxEntries: maxEntries}
}

var defaultRegistry = NewRegistry(DefaultTTL, DefaultMaxEntries)

// Resolve 用进程级状态表做归组。
func Resolve(in Input) Group { return defaultRegistry.Resolve(in) }

// Reset 清空进程级状态表（仅供测试与热重置使用）。
func Reset() { defaultRegistry.Reset() }

// Reset 清空状态表。
func (r *Registry) Reset() {
	r.mu.Lock()
	r.entries = make(map[string]*entry)
	r.mu.Unlock()
}

// Len 返回当前表内条目数。
func (r *Registry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}

// Resolve 计算一次调用的归组结果，并推进该线程的步骤计数。
func (r *Registry) Resolve(in Input) Group {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}

	anchor := anchorKey(in.FirstUserPrompt)
	base := identityKey(in)

	source := SourceNone
	sessionID := ""
	switch {
	case in.ClientSessionID != "":
		source = SourceClient
		sessionID = in.ClientSessionID
	case anchor != "":
		source = SourceAuto
	}

	grp := Group{
		SessionSource: source,
		Title:         title(in.FirstUserPrompt),
	}
	// 既没有会话头也没有锚点消息时无法归组：保持与历史一致，不产出 session。
	if source == SourceNone {
		return grp
	}

	threadKey := base + "|" + anchor
	grp.ThreadID = "th-" + shortHash(threadKey)

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.entries == nil {
		r.entries = make(map[string]*entry)
	}
	e := r.entries[threadKey]
	if e == nil || now.Sub(e.lastSeen) > r.ttl {
		if source == SourceAuto {
			sessionID = autoSessionID(threadKey, now)
		}
		e = &entry{
			sessionID: sessionID,
			lastSeen:  now,
			step:      1,
			attempt:   1,
			lastFP:    in.RequestFingerprint,
			lastFPAt:  now,
		}
		r.entries[threadKey] = e
		r.pruneLocked(now)
	} else {
		// 请求可能以乱序批次到达，时间回退时钳住，避免把步骤记到过去。
		if now.Before(e.lastSeen) {
			now = e.lastSeen
		}
		e.lastSeen = now
		if in.RequestFingerprint != "" && in.RequestFingerprint == e.lastFP && now.Sub(e.lastFPAt) <= attemptWindow {
			e.attempt++
		} else {
			e.step++
			e.attempt = 1
			e.lastFP = in.RequestFingerprint
			e.lastFPAt = now
		}
		if source == SourceAuto {
			sessionID = e.sessionID
		}
	}

	grp.SessionID = sessionID
	grp.StepIndex = e.step
	grp.AttemptIndex = e.attempt
	return grp
}

// pruneLocked 淘汰过期条目；仍超上限则整体重置（调用方已持锁）。
func (r *Registry) pruneLocked(now time.Time) {
	if len(r.entries) <= r.maxEntries {
		return
	}
	for k, e := range r.entries {
		if now.Sub(e.lastSeen) > r.ttl {
			delete(r.entries, k)
		}
	}
	if len(r.entries) > r.maxEntries {
		r.entries = make(map[string]*entry)
	}
}

// identityKey 是归组的身份维度：同一会话头/同一用户 + 同一 API Key 才算同一来源。
func identityKey(in Input) string {
	if in.ClientSessionID != "" {
		return "c:" + in.ClientSessionID
	}
	return "u:" + strconv.FormatInt(in.UserID, 10) + ":" + strconv.FormatInt(in.APIKeyID, 10)
}

// autoSessionID 生成推导会话 id：身份+锚点指纹 + 本次会话首次出现的时间。
// 身份进 hash 是为了让不同用户/不同 Key 的相同开场白落到不同会话；
// 时间后缀保证跨重启不与上一轮生成物撞车。
func autoSessionID(threadKey string, now time.Time) string {
	return "auto-" + shortHash(threadKey) + "-" + strconv.FormatInt(now.Unix(), 36)
}

// title 取锚点消息首行作为会话标题。
// 客户端常把用户原话包在上下文块（AGENTS.md / IDE 开着的文件）后面，
// 这时取「真实请求」段落，否则标题会被注入内容占满。
func title(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return ""
	}
	if section := prompttext.ExtractRequestSection(trimmed); section != "" {
		trimmed = section
	}
	if idx := strings.IndexAny(trimmed, "\r\n"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= titleLimit {
		return trimmed
	}
	cut := titleLimit
	for cut > 0 && !utf8.RuneStart(trimmed[cut]) {
		cut--
	}
	return strings.TrimSpace(trimmed[:cut]) + "…"
}

// anchorKey 是线程指纹：优先取「真实请求」段落。
// 客户端在上游做上下文压缩时会改写首条消息的注入部分，
// 但用户那句话不变；只按整条消息取指纹会把同一段对话切成两条线程。
func anchorKey(firstUser string) string {
	if section := prompttext.ExtractRequestSection(firstUser); section != "" {
		return fingerprint(section)
	}
	return fingerprint(firstUser)
}

// fingerprint 是锚点消息的短指纹；空消息返回空串。
func fingerprint(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) > fingerprintLimit {
		cut := fingerprintLimit
		for cut > 0 && !utf8.RuneStart(trimmed[cut]) {
			cut--
		}
		trimmed = trimmed[:cut]
	}
	return shortHash(trimmed)
}

func shortHash(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:4])
}
