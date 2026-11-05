package langfuse

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultQueueSize = 4096
	// defaultMaxBatchBytes 限制单批的近似字节数：开了正文采集后，
	// 64 条调用 × 32KB 正文会把单次 POST 顶到数 MB。
	defaultMaxBatchBytes = 1 << 20
	defaultBatchSize     = 64
	defaultFlushInterval = 5 * time.Second
	defaultTimeout       = 10 * time.Second

	// ingestionPath 是 Langfuse 的批量写入端点。
	ingestionPath = "/api/public/ingestion"
)

// Options 是观测通道的装配参数。
//
// 只要 Enabled 为 false，或 host/公钥/私钥任一为空，New 就返回 Nop：
// 不建队列、不起 goroutine、不持有 http.Client —— 关闭态零副作用。
type Options struct {
	Enabled       bool
	Host          string
	PublicKey     string
	SecretKey     string
	QueueSize     int
	BatchSize     int
	MaxBatchBytes int
	FlushInterval time.Duration
	Timeout       time.Duration
	Release       string
}

func (o Options) normalized() Options {
	o.Host = strings.TrimSpace(o.Host)
	o.PublicKey = strings.TrimSpace(o.PublicKey)
	o.SecretKey = strings.TrimSpace(o.SecretKey)
	if o.QueueSize <= 0 {
		o.QueueSize = defaultQueueSize
	}
	if o.BatchSize <= 0 {
		o.BatchSize = defaultBatchSize
	}
	if o.BatchSize > o.QueueSize {
		o.BatchSize = o.QueueSize
	}
	if o.MaxBatchBytes <= 0 {
		o.MaxBatchBytes = defaultMaxBatchBytes
	}
	if o.MaxBatchBytes < 8*1024 {
		o.MaxBatchBytes = 8 * 1024
	}
	if o.FlushInterval <= 0 {
		o.FlushInterval = defaultFlushInterval
	}
	if o.Timeout <= 0 {
		o.Timeout = defaultTimeout
	}
	return o
}

func (o Options) usable() bool {
	return o.Enabled && o.Host != "" && o.PublicKey != "" && o.SecretKey != ""
}

// New 按配置返回上报端。配置不完整时返回 Nop。
func New(opts Options) Sink {
	opts = opts.normalized()
	if !opts.usable() {
		return Nop()
	}
	return NewClient(opts)
}

// NewClient 无条件构造批量客户端（供测试直接接管）。
func NewClient(opts Options) *Client {
	opts = opts.normalized()
	c := &Client{
		opts:     opts,
		endpoint: strings.TrimRight(opts.Host, "/") + ingestionPath,
		auth: "Basic " + base64.StdEncoding.EncodeToString(
			[]byte(opts.PublicKey+":"+opts.SecretKey)),
		queue:      make(chan Call, opts.QueueSize),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		httpClient: &http.Client{Timeout: opts.Timeout},
	}
	go c.run()
	return c
}

// Client 是通过有界队列批量上报的 Sink 实现。
type Client struct {
	opts     Options
	endpoint string
	auth     string

	httpClient *http.Client
	queue      chan Call
	stop       chan struct{}
	done       chan struct{}

	stopped sync.Once
	closed  atomic.Bool
	dropped atomic.Uint64
	sent    atomic.Uint64
	batches atomic.Uint64
	failed  atomic.Uint64
}

// Observe 投递一条调用记录。队列满即丢，绝不阻塞调用方。
func (c *Client) Observe(call Call) {
	if c == nil || c.closed.Load() {
		return
	}
	select {
	case c.queue <- call:
	default:
		c.dropped.Add(1)
	}
}

// Dropped 返回因队列满被丢弃的条数（仅内存计数，不落日志）。
func (c *Client) Dropped() uint64 { return c.dropped.Load() }

// Sent 返回已成功上报的条数。
func (c *Client) Sent() uint64 { return c.sent.Load() }

// Batches 返回已发出的批次数。
func (c *Client) Batches() uint64 { return c.batches.Load() }

// Failed 返回上报失败的批次数。
func (c *Client) Failed() uint64 { return c.failed.Load() }

// Close 停止接收新记录并冲刷剩余批次。
func (c *Client) Close(ctx context.Context) error {
	if c == nil {
		return nil
	}
	c.stopped.Do(func() {
		c.closed.Store(true)
		close(c.stop)
	})
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) run() {
	defer close(c.done)
	// worker 内部吞掉一切 panic：观测通道的任何故障都不得传染到进程或业务 goroutine。
	defer func() { _ = recover() }()

	ticker := time.NewTicker(c.opts.FlushInterval)
	defer ticker.Stop()

	batch := make([]Call, 0, c.opts.BatchSize)
	batchBytes := 0
	for {
		select {
		case call := <-c.queue:
			batch, batchBytes = c.push(batch, batchBytes, call)
		case <-ticker.C:
			if len(batch) > 0 {
				c.flush(batch)
				batch = batch[:0]
				batchBytes = 0
			}
		case <-c.stop:
			// 退出前把队列里剩下的和手里的批次一起发掉；发送次数受 batchSize 限制，
			// 不额外重试，避免退出路径被远端拖住。
			for {
				select {
				case call := <-c.queue:
					batch, batchBytes = c.push(batch, batchBytes, call)
				default:
					if len(batch) > 0 {
						c.flush(batch)
					}
					return
				}
			}
		}
	}
}

func (c *Client) flush(calls []Call) {
	if len(calls) == 0 {
		return
	}
	defer func() { _ = recover() }()

	payload, err := json.Marshal(buildBatch(calls, c.opts.Release))
	if err != nil {
		c.failed.Add(1)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.opts.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		c.failed.Add(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.auth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.failed.Add(1)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	// 读掉响应体以便连接复用；内容不解析、不记录。
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		c.failed.Add(1)
		return
	}
	c.batches.Add(1)
	c.sent.Add(uint64(len(calls)))
}

// push 把一条调用并入当前批次；条数或字节数任一超限就先冲刷。
func (c *Client) push(batch []Call, batchBytes int, call Call) ([]Call, int) {
	size := approxCallBytes(call)
	if len(batch) > 0 && (len(batch) >= c.opts.BatchSize || batchBytes+size > c.opts.MaxBatchBytes) {
		c.flush(batch)
		return []Call{call}, size
	}
	return append(batch, call), batchBytes + size
}

// approxCallBytes 估一条调用占用的事件体大小（正文 + 固定字段开销）。
func approxCallBytes(call Call) int {
	const overhead = 512
	return len(call.Input) + len(call.Output) + overhead
}
