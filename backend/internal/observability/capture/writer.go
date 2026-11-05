package capture

import (
	"github.com/gin-gonic/gin"
)

// writer 把响应字节旁路复制给 recorder，同时保持原 writer 的全部语义。
type writer struct {
	gin.ResponseWriter
	rec *Recorder
}

func (w *writer) Write(p []byte) (int, error) {
	w.rec.AppendResponse(p)
	return w.ResponseWriter.Write(p)
}

func (w *writer) WriteString(s string) (int, error) {
	w.rec.AppendResponse([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

// Install 在请求上安装正文采集：包装 c.Writer、把 recorder 挂到请求 ctx。
// 返回 recorder 与还原函数；调用方必须 defer 还原，避免包装泄漏到后续中间件。
func Install(c *gin.Context, opts Options) (*Recorder, func()) {
	if c == nil || c.Writer == nil || !opts.Enabled() {
		return nil, func() {}
	}
	rec := New(opts)
	original := c.Writer
	wrapped := &writer{ResponseWriter: original, rec: rec}
	c.Writer = wrapped
	if c.Request != nil {
		c.Request = c.Request.WithContext(With(c.Request.Context(), rec))
	}
	return rec, func() {
		// 只在仍然是我们这一层时还原；被下游再包一层时不动。
		if c.Writer == wrapped {
			c.Writer = original
		}
	}
}
