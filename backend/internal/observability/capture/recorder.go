// Package capture 采集单次模型调用的提示词与响应正文，供观测层落正文。
//
// 三条约束：
//   - 只在装配了 recorder 的请求上工作；未安装时零开销（handler 不包 writer、不分配缓冲）；
//   - 缓冲有硬上限，超出即截断并标记，避免大响应把内存吃穿；
//   - 不落本地文件、不写应用日志：采到的正文只随观测上报出网。
package capture

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"sync"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/prompttext"
)

// maxPendingBytes 是未成行 SSE 尾巴的上限，防止巨型无换行载荷撑爆缓冲。
const maxPendingBytes = 64 * 1024

// 请求指纹只取首尾定长片段：会话里每一步的差异都落在尾部（追加的工具结果），
// 头部（system/工具定义）逐轮不变。这样指纹成本是常数级，不受请求体大小影响。
const (
	fingerprintHeadBytes = 2 * 1024
	fingerprintTailBytes = 8 * 1024
)

// DefaultMaxBytes 是单侧（请求/响应）正文的默认上限。
const DefaultMaxBytes = 32 * 1024

// defaultPromptMaxBytes 是用户消息文本的默认上限。零值 Options 不能等于"无上限"，
// 否则一个漏配的调用点会把整份请求体留在内存里再发出去。
const defaultPromptMaxBytes = 512 * 1024

// Options 是采集开关。两项都关时不安装 recorder，链路零开销。
type Options struct {
	Prompt bool
	// PromptMaxBytes 是「用户消息文本」的上限；<=0 表示不截断（由配置层钳制）。
	// 它与 MaxBytes 分开：请求体里塞满了工具输出，正文往往比 body 小两个数量级。
	PromptMaxBytes int
	Response       bool
	MaxBytes       int
}

// Enabled 报告是否需要安装采集。
func (o Options) Enabled() bool { return o.Prompt || o.Response }

// Recorder 保存单次调用的请求正文与响应原始字节。
type Recorder struct {
	mu           sync.Mutex
	limit        int
	promptCap    int
	wantReq      bool
	wantResp     bool
	request      string
	reqTruncated bool
	resp         []byte
	// pending 保存尚未成行的 SSE 尾巴；text 是边写边抽出来的 assistant 文本。
	// 之所以要增量抽取：Responses 的正文 delta 排在几十 KB 的 response.created
	// 前导之后，只靠缓冲末尾统一解析，一旦触发上限就永远看不到正文。
	pending             []byte
	text                []byte
	textFull            bool
	sawDelta            bool
	userText            string
	userTurns           int
	userFirst           string
	userLatest          string
	userLatestTruncated bool
	userHistoryBytes    int
	reqFP               string
	protocol            string
	promptStat          string
	// respTruncated 标记**原始响应字节**达到上限被截；它与正文是否被截是两件事：
	// 流里有 response.created 这类大 JSON，原始字节几乎必然超上限，
	// 但正文往往远未触顶。两者混用会把每一条都标成"输出被截"。
	respTruncated bool
	// finishReason 记录上游给出的收尾原因，用来区分"模型自己截断"和"我们截断"。
	finishReason string
}

// New 按选项构造 recorder。MaxBytes<=0 时用默认值。
func New(opts Options) *Recorder {
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = DefaultMaxBytes
	}
	if opts.PromptMaxBytes <= 0 {
		opts.PromptMaxBytes = defaultPromptMaxBytes
	}
	return &Recorder{
		limit:     opts.MaxBytes,
		promptCap: opts.PromptMaxBytes,
		wantReq:   opts.Prompt,
		wantResp:  opts.Response,
	}
}

// SetRequest 记录请求正文（已按上限截断）。
func (r *Recorder) SetRequest(text string) {
	if r == nil || !r.wantReq {
		return
	}
	fp := requestFingerprint(text)
	r.mu.Lock()
	r.request, r.reqTruncated = truncate(text, r.limit)
	r.reqFP = fp
	r.mu.Unlock()

	// 单独抽一份「用户说的话」：观测层默认展示这一份，
	// 原始请求体退到 metadata 里备查。
	extracted := prompttext.ExtractWithLimit([]byte(text), r.promptCap)
	r.mu.Lock()
	r.protocol = extracted.Protocol
	r.promptStat = extracted.Status
	r.userFirst = extracted.FirstUser
	r.userLatest = extracted.LatestUser
	r.userLatestTruncated = extracted.LatestTruncated
	r.userHistoryBytes = extracted.HistoryBytes
	if extracted.Text != "" {
		r.userText = extracted.Text
		r.userTurns = extracted.Turns
	}
	r.mu.Unlock()
}

// FirstUserPrompt 返回用于会话归组的锚点消息（首条真实用户消息）。
func (r *Recorder) FirstUserPrompt() string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.userFirst
}

// LatestUserPrompt 返回最新一条真正的用户消息：观测层的 input 用它，
// 否则长会话下 input 会被几十轮历史占满，真正被回答的那句反而沉到底部。
func (r *Recorder) LatestUserPrompt() string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.userLatest
}

// LatestUserTruncated 报告展示用的用户消息是否被上限截断。
func (r *Recorder) LatestUserTruncated() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.userLatestTruncated
}

// UserPromptHistoryBytes 返回本次请求携带的完整用户历史字节数（截断前）。
func (r *Recorder) UserPromptHistoryBytes() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.userHistoryBytes
}

// RequestFingerprint 返回请求体指纹，供观测层识别「同一步的重复尝试」。
func (r *Recorder) RequestFingerprint() string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reqFP
}

// RequestTruncated 报告请求正文是否被截断。
func (r *Recorder) RequestTruncated() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reqTruncated
}

// RequestText 返回请求正文。
func (r *Recorder) RequestText() string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.request
}

// AppendResponse 旁路累积响应字节。
func (r *Recorder) AppendResponse(p []byte) {
	if r == nil || !r.wantResp || len(p) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// 原始字节只留作解析失败时的兜底，按上限截断。
	if len(r.resp) < r.limit {
		remain := r.limit - len(r.resp)
		if len(p) > remain {
			r.resp = append(r.resp, p[:remain]...)
			r.respTruncated = true
		} else {
			r.resp = append(r.resp, p...)
		}
	} else {
		r.respTruncated = true
	}

	// 增量抽取：按行切，逐条 data: 事件取正文。
	r.pending = append(r.pending, p...)
	for {
		idx := bytes.IndexByte(r.pending, '\n')
		if idx < 0 {
			break
		}
		line := r.pending[:idx]
		r.pending = r.pending[idx+1:]
		piece, fullText, reason := parseSSELine(line)
		if reason != "" {
			r.finishReason = reason
		}
		if piece == "" {
			continue
		}
		// 同一个流里两种事件可能带同一段文本（delta 增量 + done 全文），
		// 已经用增量拼出来之后，全文事件直接丢弃，避免正文出现两份。
		if fullText {
			if r.sawDelta || len(r.text) > 0 {
				continue
			}
		} else {
			r.sawDelta = true
		}
		r.appendTextLocked(piece)
	}
	// 单行异常巨大（无换行的巨型 JSON）时丢弃尾巴，避免 pending 无界增长。
	if len(r.pending) > maxPendingBytes {
		r.pending = r.pending[:0]
	}
}

// flushPendingLocked 把还没等到换行符的最后一段解析掉。
// 流结束时上游不一定补换行，留在 pending 里的那一行就是正文的最后一段，
// 不解析等于固定丢掉结尾。调用方必须持锁。
func (r *Recorder) flushPendingLocked() {
	if len(r.pending) == 0 {
		return
	}
	line := r.pending
	r.pending = nil
	piece, fullText, reason := parseSSELine(line)
	if reason != "" {
		r.finishReason = reason
	}
	if piece == "" {
		return
	}
	if fullText && (r.sawDelta || len(r.text) > 0) {
		return
	}
	if !fullText {
		r.sawDelta = true
	}
	r.appendTextLocked(piece)
}

// appendTextLocked 追加抽到的正文片段，超过上限即标记并停止累积。
func (r *Recorder) appendTextLocked(piece string) {
	if piece == "" || r.textFull {
		return
	}
	if len(r.text)+len(piece) > r.limit {
		remain := r.limit - len(r.text)
		if remain > 0 {
			// 按字节硬切会切掉半个汉字，JSON 编码后变成 U+FFFD；
			// 回退到 rune 边界再拼。
			cut := remain
			for cut > 0 && !utf8.RuneStart(piece[cut]) {
				cut--
			}
			r.text = append(r.text, piece[:cut]...)
		}
		r.textFull = true
		return
	}
	r.text = append(r.text, piece...)
}

// ResponseRaw 返回累积的响应原始字节副本。
func (r *Recorder) ResponseRaw() []byte {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]byte(nil), r.resp...)
}

// TextTruncated 报告**正文**是否因为缓冲上限被截断。
// 只有它才代表"输出不全"；原始字节被截不算。
func (r *Recorder) TextTruncated() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.textFull
}

// RawTruncated 报告**原始响应字节**是否被截断（用于排查解析兜底是否还能用）。
func (r *Recorder) RawTruncated() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.respTruncated
}

// FinishReason 返回上游给的收尾原因（stop / length / tool_calls / incomplete:*）。
// 空串表示没抓到，调用方不要把空串当成"正常结束"。
func (r *Recorder) FinishReason() string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.finishReason
}

type ctxKey struct{}

// With 把 recorder 挂到 ctx 上，供计费链路末端取用。
func With(ctx context.Context, r *Recorder) context.Context {
	if r == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, r)
}

// From 取出 ctx 上的 recorder，未安装时返回 nil。
func From(ctx context.Context) *Recorder {
	if ctx == nil {
		return nil
	}
	r, _ := ctx.Value(ctxKey{}).(*Recorder)
	return r
}

// truncate 按字节上限截断，并回退到 rune 边界：
// 从中文中间切断会产出非法 UTF-8，JSON 编码时被替换成 U+FFFD，正文就花了。
func truncate(s string, limit int) (string, bool) {
	if len(s) <= limit {
		return s, false
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], true
}

// UserPrompt 返回抽取出的用户消息文本、条数与识别到的协议。
func (r *Recorder) UserPrompt() (text string, turns int, protocol, status string) {
	if r == nil {
		return "", 0, "", ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.userText, r.userTurns, r.protocol, r.promptStat
}

// ResponseText 返回响应正文的 assistant 文本。
//
// 顺序：增量抽取的文本 → 对完整缓冲做一次结构化解析（覆盖非流式 JSON）→ 原始字节兜底。
// 兜底保证"采到的正文"不会因为解析不认识而丢失。
func (r *Recorder) ResponseText() string {
	if r == nil || !r.wantResp {
		return ""
	}
	r.mu.Lock()
	// 流结束时上游不一定补换行，pending 里那一行就是正文的最后一段。
	r.flushPendingLocked()
	text := string(r.text)
	raw := append([]byte(nil), r.resp...)
	r.mu.Unlock()
	if text != "" {
		return text
	}

	if len(raw) == 0 {
		return ""
	}
	// 非流式响应没有 SSE 收尾事件，只能从这里补 finish reason。
	r.finalize(raw)
	if parsed, ok := ExtractAssistantText(raw); ok && parsed != "" {
		return parsed
	}
	// 已经是 SSE 流却没抽到文本：说明这一轮没有 assistant 文本（例如纯工具调用），
	// 此时回退原始字节只会把 event:/data: 前导当成正文，宁可留空。
	if looksLikeSSE(string(raw)) {
		return ""
	}
	return string(raw)
}

// finalize 在响应读取完成后补最后一件事：非流式响应体里的收尾原因。
func (r *Recorder) finalize(raw []byte) {
	if r == nil {
		return
	}
	r.mu.Lock()
	missing := r.finishReason == ""
	r.mu.Unlock()
	if !missing {
		return
	}
	if reason := FinishReasonFromJSON(raw); reason != "" {
		r.mu.Lock()
		r.finishReason = reason
		r.mu.Unlock()
	}
}

// requestFingerprint 计算请求体的短指纹，用于观测层区分「同一步的重复尝试」
// 与「新的一步」。取首尾定长片段 + 总长度，成本与请求体大小无关。
func requestFingerprint(text string) string {
	if text == "" {
		return ""
	}
	h := sha1.New()
	if len(text) <= fingerprintHeadBytes+fingerprintTailBytes {
		_, _ = h.Write([]byte(text))
	} else {
		_, _ = h.Write([]byte(text[:fingerprintHeadBytes]))
		_, _ = h.Write([]byte(text[len(text)-fingerprintTailBytes:]))
	}
	_, _ = h.Write([]byte(strconv.Itoa(len(text))))
	return hex.EncodeToString(h.Sum(nil)[:8])
}
