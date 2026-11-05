// Package sessiongroup 把同一会话下的多次模型调用归并到一个稳定单元，
// 供观测层在 Langfuse 侧按会话聚合、按轮次成卡、按线程读完整对话。
//
// 五条约束：
//   - 纯内存：只维护锚点/轮次的步骤小表，不产生任何 I/O、不写日志、不落盘；
//   - 只是读取：不感知计费、不改变对外行为，也不回写任何业务字段；
//   - 客户端显式会话头优先，缺失时才用「用户身份 + 首条用户消息」推导；
//   - 有界：表大小超限先淘汰过期项，仍超限则整体重置，宁可再生步骤编号也不吃内存；
//   - 两层分离：会话/线程跨轮次稳定（会话聚合键），轮次态只管轮内步序，
//     因为一轮对话里的几十次上游调用共享同一个用户问题，混在一起才是错的。
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
	// DefaultTTL 是线程锚点的空闲过期时间：超过它再出现即视为新会话
	// （推导 id 重新生成）。
	DefaultTTL = 30 * time.Minute

	// DefaultTurnIdle 是轮次的空闲过期时间：该轮最后一次步骤过去这么久之后
	// 再收到同样的轮次签名，就当作新一轮重新开步序。
	// 与观测层落卡的收尾阈值保持一致，否则会出现「卡片已收尾、步序还在累加」。
	DefaultTurnIdle = 5 * time.Minute

	// DefaultMaxEntries 是单张表的内存上限。
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
	// UserTurns 是请求里用户消息的条数。
	//
	// 它是轮次识别的唯一依据：agentic 客户端（Codex / Claude Code 这类）在一轮
	// 对话里只让用户开口一次，后续步骤全是工具结果，条数恒定；用户再开口才 +1。
	// 实测同一线程内该值稳定（3,3,3 / 14,14,14），所以「条数」比「是否前进」
	// 更适合做轮次键 —— 并发子代理会让条数在 3 和 14 之间来回跳。
	UserTurns int
	// RequestFingerprint 是请求体指纹，用于识别「同一步的重复尝试」。
	RequestFingerprint string
	// TurnIdle 是轮次空闲阈值。观测层必须与自己的落卡阈值传同一个值，
	// 否则会出现「卡片已收尾、步序还在往下累加」。非正值用 Registry 默认值。
	TurnIdle time.Duration
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
	// TurnID 是一轮的稳定标识：同一轮里所有上游调用共享同一个值。
	// 观测层用它做卡片聚合键，因此它必须只由「会话 + 线程 + 轮次签名」决定。
	TurnID string
	// TurnKnown 报告轮次签名是否可信。为 false 时（抽不到用户消息条数）
	// TurnID 仍然给出（全会话一个桶），但上层**不应该**拿它聚合卡片：
	// 那会把连续 N 轮并成一张内容错乱的大卡，比「一调用一张」更糟。
	TurnKnown bool
	// TurnIndex 是线程内的轮次序号（从 1 开始），只用于展示。
	TurnIndex int
	// StepIndex 是**轮内**第几次上游请求（从 1 开始）。
	// 旧实现是「线程内第几次」，把一轮几十步摊平成一条长列表，卡片无法阅读。
	StepIndex int
	// AttemptIndex 是同一轮内同一步的第几次尝试（从 1 开始，>1 表示客户端重试/并行）。
	AttemptIndex int
	// Title 是会话标题，取锚点消息首行。
	Title string
}

// threadState 是线程级状态：只负责跨轮次稳定的会话聚合键。
type threadState struct {
	sessionID string
	lastSeen  time.Time
}

// turnState 是轮次级状态：轮内步序与重试折叠，轮次之间互不影响。
type turnState struct {
	steps    int
	attempt  int
	lastFP   string
	lastFPAt time.Time
	lastSeen time.Time
}

// Registry 是归组状态表。零值不可用，用 NewRegistry 构造。
type Registry struct {
	mu         sync.Mutex
	threads    map[string]*threadState
	turns      map[string]*turnState
	ttl        time.Duration
	turnIdle   time.Duration
	maxEntries int
}

// NewRegistry 构造状态表。ttl / turnIdle / maxEntries 非正值时用默认值。
func NewRegistry(ttl time.Duration, turnIdle time.Duration, maxEntries int) *Registry {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if turnIdle <= 0 {
		turnIdle = DefaultTurnIdle
	}
	if maxEntries <= 0 {
		maxEntries = DefaultMaxEntries
	}
	return &Registry{
		threads:    make(map[string]*threadState),
		turns:      make(map[string]*turnState),
		ttl:        ttl,
		turnIdle:   turnIdle,
		maxEntries: maxEntries,
	}
}

var defaultRegistry = NewRegistry(DefaultTTL, DefaultTurnIdle, DefaultMaxEntries)

// Resolve 用进程级状态表做归组。
func Resolve(in Input) Group { return defaultRegistry.Resolve(in) }

// Reset 清空进程级状态表（仅供测试与热重置使用）。
func Reset() { defaultRegistry.Reset() }

// Reset 清空状态表。
func (r *Registry) Reset() {
	r.mu.Lock()
	r.threads = make(map[string]*threadState)
	r.turns = make(map[string]*turnState)
	r.mu.Unlock()
}

// Len 返回当前表内条目数（线程 + 轮次）。
func (r *Registry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.threads) + len(r.turns)
}

// Resolve 计算一次调用的归组结果，并推进该轮次的步骤计数。
func (r *Registry) Resolve(in Input) Group {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	in.ClientSessionID = clampSessionID(in.ClientSessionID)

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

	idle := in.TurnIdle
	if idle <= 0 {
		idle = r.turnIdle
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.threads == nil {
		r.threads = make(map[string]*threadState)
	}
	if r.turns == nil {
		r.turns = make(map[string]*turnState)
	}

	// 线程态：维护会话聚合键，跨轮次不变。
	ts := r.threads[threadKey]
	if ts == nil || now.Sub(ts.lastSeen) > r.ttl {
		if source == SourceAuto {
			sessionID = autoSessionID(threadKey, now)
		}
		ts = &threadState{sessionID: sessionID, lastSeen: now}
		r.threads[threadKey] = ts
		r.pruneLocked(now)
	} else {
		ts.lastSeen = now
		if source == SourceAuto {
			sessionID = ts.sessionID
		}
	}

	// 上下文压缩会让用户消息条数回退（14 → 3），理论上会撞上压缩前那一代的
	// 同名键；但**不能**用"条数回退即换代"来兜：同一线程里并发子代理的条数
	// 本来就在 3 和 14 之间来回跳（实测形态就是 3,3,3,14,3,14…），
	// 那样每来一条就换一次代，卡片会被彻底切碎。
	// 压缩回退撞键的代价只是步数接着旧值往下数、卡片偏大，不丢数据；
	// 交替误判的代价是每步一张卡 —— 两害相权取其轻，这里只保留条数做键。

	turnKey := threadKey + "|" + turnSignature(in)
	grp.TurnID = "tu-" + shortHash(turnKey)
	grp.TurnIndex = turnIndex(in)
	grp.TurnKnown = in.UserTurns > 0

	// 轮次态：负责轮内步序与重试折叠。
	tu := r.turns[turnKey]
	if tu == nil || now.Sub(tu.lastSeen) > idle {
		tu = &turnState{
			steps:    1,
			attempt:  1,
			lastFP:   in.RequestFingerprint,
			lastFPAt: now,
			lastSeen: now,
		}
		r.turns[turnKey] = tu
		r.pruneLocked(now)
	} else {
		// 请求可能以乱序批次到达，时间回退时钳住，避免把步骤记到过去。
		if now.Before(tu.lastSeen) {
			now = tu.lastSeen
		}
		tu.lastSeen = now
		if in.RequestFingerprint != "" && in.RequestFingerprint == tu.lastFP && now.Sub(tu.lastFPAt) <= attemptWindow {
			tu.attempt++
		} else {
			tu.steps++
			tu.attempt = 1
			tu.lastFP = in.RequestFingerprint
			tu.lastFPAt = now
		}
	}

	grp.SessionID = sessionID
	grp.StepIndex = tu.steps
	grp.AttemptIndex = tu.attempt
	return grp
}

// sessionIDLimit 是客户端会话标识的字节上限。
//
// SessionID 来自请求头（session_id / X-Session-Id / X-Claude-Code-Session-Id），
// 而 Go 的 MaxHeaderBytes 默认 1MB：不钳住的话它可以同时成为常驻的 map key
// 和 trace 上的 sessionId 字段，一张卡片就能把上报批次顶穿。
const sessionIDLimit = 256

// clampSessionID 收口客户端会话标识：截断超长值、抹掉控制字符，
// 并附上全量短指纹保证截断后仍能区分。
func clampSessionID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// 给 "#"+8 位指纹留出位置，保证最终长度不超过上限。
	keep := sessionIDLimit - 9
	if len(s) <= keep && isCleanSessionID(s) {
		return s
	}
	base := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, truncateRunes(s, keep))
	return base + "#" + shortHash(s)
}

// isCleanSessionID 报告字符串能否原样用作会话键（不含控制字符）。
func isCleanSessionID(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return false
		}
	}
	return true
}

// turnSignature 是轮次签名。
//
// 只用「用户消息条数」：它是本轮对话的身份，随用户开口单调 +1，不随工具调用变化。
// 不能把「最新一条用户消息」也算进去 —— 部分客户端会把工具结果包成 user 消息，
// 那会让同一轮在每一步都换签名，一张卡被拆成几十张，正是要修的病。
func turnSignature(in Input) string {
	if in.UserTurns > 0 {
		return "t" + strconv.Itoa(in.UserTurns)
	}
	// 抽不到条数时退化为单桶：靠空闲超时切分，至少不会把不同轮次硬拼成一张卡。
	return "t0"
}

// turnIndex 是轮次序号，仅用于展示。
func turnIndex(in Input) int {
	if in.UserTurns > 0 {
		return in.UserTurns
	}
	return 1
}

// pruneLocked 淘汰过期条目；仍超上限则整体重置（调用方已持锁）。
func (r *Registry) pruneLocked(now time.Time) {
	if len(r.threads) <= r.maxEntries && len(r.turns) <= r.maxEntries {
		return
	}
	for k, ts := range r.threads {
		if now.Sub(ts.lastSeen) > r.ttl {
			delete(r.threads, k)
		}
	}
	for k, tu := range r.turns {
		if now.Sub(tu.lastSeen) > r.turnIdle {
			delete(r.turns, k)
		}
	}
	if len(r.threads) > r.maxEntries {
		r.threads = make(map[string]*threadState)
	}
	if len(r.turns) > r.maxEntries {
		r.turns = make(map[string]*turnState)
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
	line, truncated := titleLine(trimmed)
	if line == "" {
		return ""
	}
	if truncated {
		return line + "…"
	}
	return line
}

// titleLine 取第一条非空行，并按 titleLimit 截断。
// 它同时是会话标题与线程指纹的来源：人能读的那句话就该决定归组。
func titleLine(text string) (string, bool) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		if len(line) > titleLimit {
			return truncateRunes(line, titleLimit), true
		}
		return line, false
	}
	return "", false
}

// truncateRunes 按字节上限截断并回退到 rune 边界。
func truncateRunes(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// anchorKey 是线程指纹：优先取「真实请求」段落。
// 客户端在上游做上下文压缩时会改写首条消息的注入部分，
// 但用户那句话不变；只按整条消息取指纹会把同一段对话切成两条线程。
func anchorKey(firstUser string) string {
	text := strings.TrimSpace(firstUser)
	if section := prompttext.ExtractRequestSection(text); section != "" {
		text = section
	}
	// 只取首行：上游压缩上下文时会重写首条消息的后续部分，
	// 但"这件事是什么"那句话不变；按整条取指纹会把一段对话切成几节。
	line, _ := titleLine(text)
	return fingerprint(line)
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
