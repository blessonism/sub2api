// Package capture 采集单次模型调用的提示词与响应正文，供观测层落正文。
//
// 三条约束：
//   - 只在装配了 recorder 的请求上工作；未安装时零开销（handler 不包 writer、不分配缓冲）；
//   - 缓冲有硬上限，超出即截断并标记，避免大响应把内存吃穿；
//   - 不落本地文件、不写应用日志：采到的正文只随观测上报出网。
package capture

import (
	"context"
	"sync"
	"unicode/utf8"
)

// DefaultMaxBytes 是单侧（请求/响应）正文的默认上限。
const DefaultMaxBytes = 32 * 1024

// Options 是采集开关。两项都关时不安装 recorder，链路零开销。
type Options struct {
	Prompt   bool
	Response bool
	MaxBytes int
}

// Enabled 报告是否需要安装采集。
func (o Options) Enabled() bool { return o.Prompt || o.Response }

// Recorder 保存单次调用的请求正文与响应原始字节。
type Recorder struct {
	mu           sync.Mutex
	limit        int
	wantReq      bool
	wantResp     bool
	request      string
	reqTruncated bool
	resp         []byte
	// respTruncated 标记响应字节已达到上限被截断。
	respTruncated bool
}

// New 按选项构造 recorder。MaxBytes<=0 时用默认值。
func New(opts Options) *Recorder {
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = DefaultMaxBytes
	}
	return &Recorder{limit: opts.MaxBytes, wantReq: opts.Prompt, wantResp: opts.Response}
}

// SetRequest 记录请求正文（已按上限截断）。
func (r *Recorder) SetRequest(text string) {
	if r == nil || !r.wantReq {
		return
	}
	r.mu.Lock()
	r.request, r.reqTruncated = truncate(text, r.limit)
	r.mu.Unlock()
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
	if len(r.resp) >= r.limit {
		r.respTruncated = true
		return
	}
	remain := r.limit - len(r.resp)
	if len(p) > remain {
		r.resp = append(r.resp, p[:remain]...)
		r.respTruncated = true
		return
	}
	r.resp = append(r.resp, p...)
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

// ResponseTruncated 报告响应是否被截断。
func (r *Recorder) ResponseTruncated() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.respTruncated
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

// ResponseText 返回响应正文的 assistant 文本。
// 结构化解析失败时回退为原始字节，保证"采到的正文"不会因为解析不认识而丢失。
func (r *Recorder) ResponseText() string {
	if r == nil || !r.wantResp {
		return ""
	}
	raw := r.ResponseRaw()
	if len(raw) == 0 {
		return ""
	}
	if text, ok := ExtractAssistantText(raw); ok && text != "" {
		return text
	}
	return string(raw)
}
