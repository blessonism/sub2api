package langfuse

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ErrWorkerAborted 表示上报 worker 因 panic 提前退出（不写日志，只回报给调用方）。
var ErrWorkerAborted = errors.New("langfuse: ingest worker aborted")

const (
	// exitFlushBudget 是退出时留给「把卡片发出去」的时间预算。
	// 它比 main 传给 Shutdown 的总预算略小，好让 worker 自己在预算内收口，
	// 而不是被 ctx 掐断后连一张卡片都没发。
	exitFlushBudget = 4 * time.Second

	// maxIngestionRespBytes 是读取响应体的上限，只为解析 207 的 errors[]。
	maxIngestionRespBytes = 1 << 20
)

const (
	defaultQueueSize = 4096
	// defaultMaxBatchBytes 限制单批事件的近似字节数。开了正文采集后，
	// 单步响应正文可达 1MB，64 条事件不设字节闸门会把单次 POST 顶到数 MB。
	defaultMaxBatchBytes = 1 << 20
	defaultBatchSize     = 64
	defaultFlushInterval = 5 * time.Second
	defaultTimeout       = 10 * time.Second

	// maxSendBytes 是单次 POST 的事件体估算上限。
	// Langfuse 的硬限是 4.5MB（实测 8MB 直接 413 "Body exceeded 4.5mb limit"），
	// 留出 JSON 转义膨胀与字段开销的余量后按 3MB 收口。
	maxSendBytes = 3 << 20

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

	// TurnIdle 是轮次收尾的空闲阈值：一轮对话最后一次步骤结束这么久之后才落卡。
	// 非正值取 DefaultTurnIdle。
	TurnIdle time.Duration
	// MaxTurns 是同时未收尾的轮次上限，非正值取 DefaultMaxTurns。
	MaxTurns int
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
	if o.MaxBatchBytes > maxSendBytes {
		o.MaxBatchBytes = maxSendBytes
	}
	if o.FlushInterval <= 0 {
		o.FlushInterval = defaultFlushInterval
	}
	if o.Timeout <= 0 {
		o.Timeout = defaultTimeout
	}
	if o.TurnIdle <= 0 {
		o.TurnIdle = DefaultTurnIdle
	}
	if o.MaxTurns <= 0 {
		o.MaxTurns = DefaultMaxTurns
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
	fatal   atomic.Bool
	// exitDeadline 是退出冲刷的绝对截止时刻（UnixNano，0 表示非退出路径）。
	// 单请求超时要按它收口，否则 main 的总预算一过，worker 还在等一次 10s 的 POST。
	exitDeadline atomic.Int64
	dropped      atomic.Uint64
	sent         atomic.Uint64
	batches      atomic.Uint64
	failed       atomic.Uint64
}

// requestTimeout 返回本次 POST 可用的超时：退出路径上要按剩余预算收口。
func (c *Client) requestTimeout() time.Duration {
	timeout := c.opts.Timeout
	if ns := c.exitDeadline.Load(); ns != 0 {
		if left := time.Until(time.Unix(0, ns)); left < timeout {
			timeout = left
		}
	}
	return timeout
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

// Sent 返回已成功上报的事件条数。
func (c *Client) Sent() uint64 { return c.sent.Load() }

// Batches 返回已发出的批次数。
func (c *Client) Batches() uint64 { return c.batches.Load() }

// Failed 返回上报失败的批次数。
func (c *Client) Failed() uint64 { return c.failed.Load() }

// Close 停止接收新记录并冲刷剩余批次与未收尾的轮次卡片。
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
		// worker 因 panic 中止时如实回报，别让"上报静默死亡"看起来像正常退出。
		if c.fatal.Load() {
			return ErrWorkerAborted
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// tickInterval 是 worker 的扫描周期。
//
// 它同时负责冲刷待发事件和收尾空闲轮次，所以不能只按 FlushInterval 取大值：
// 若把 flush_interval_seconds 配成 10 分钟而 turn_idle_seconds 是 5 分钟，
// 卡片会白白多等一个周期。这里按 turnIdle/4 收口，下限 1s。
func (c *Client) tickInterval() time.Duration {
	interval := c.opts.FlushInterval
	if quarter := c.opts.TurnIdle / 4; quarter > 0 && quarter < interval {
		interval = quarter
	}
	if interval < time.Second {
		interval = time.Second
	}
	return interval
}

func (c *Client) run() {
	defer close(c.done)
	// worker 内部吞掉一切 panic：观测通道的任何故障都不得传染到进程或业务 goroutine。
	// 但不能静默死亡 —— 置位后 Close 会把这件事报给调用方（仍然不写日志、不落盘）。
	defer func() {
		if r := recover(); r != nil {
			c.fatal.Store(true)
			c.failed.Add(1)
		}
	}()

	ticker := time.NewTicker(c.tickInterval())
	defer ticker.Stop()

	buf := newTurnBuffer(c.opts.Release, c.opts.TurnIdle, c.opts.MaxTurns)
	pending := make([]event, 0, c.opts.BatchSize)
	pendingBytes := 0

	// reset 断开已发事件对正文字符串的引用：
	// 只做 pending[:0] 会让底层数组一直钉住上一批最多 64 条事件的正文字段，
	// 在下一批攒满之前 GC 都收不掉。
	reset := func() {
		clear(pending)
		pending = pending[:0]
		pendingBytes = 0
	}

	// absorb 把新事件并入待发批次，超限就先发。
	absorb := func(evs []event) {
		pending = append(pending, evs...)
		pendingBytes += eventsBytes(evs)
		if len(pending) >= c.opts.BatchSize || pendingBytes >= c.opts.MaxBatchBytes {
			c.send(pending)
			reset()
		}
	}

	for {
		select {
		case call := <-c.queue:
			// generation 立刻进批次；对应卡片留在 buf 里，等这一轮结束。
			absorb(buf.add(call))
		case <-ticker.C:
			// 收尾空闲超时的轮次：这是卡片唯一的正常出口。
			absorb(buf.expire(time.Now()))
			if len(pending) > 0 {
				c.send(pending)
				reset()
			}
		case <-c.stop:
			// 退出路径单独一套时间预算：main 的 ctx 只等几秒，而单次请求
			// 默认允许多达 10s。不收口的话，第一张卡片还没发出去进程就没了。
			c.exitDeadline.Store(time.Now().Add(exitFlushBudget).UnixNano())
			// 卡片先落：它是聚合结果，只有退出这一次机会，价值高于队列里
			// 尚未发出的步骤。先发卡片还能让它们挂上已经到达的 generation。
			absorb(buf.drain())
			if len(pending) > 0 {
				c.send(pending)
				reset()
			}
			// 再把队列里剩下的记录吸完；这些步骤即使挂不上 trace 也只是不可见，
			// 不会污染已经发出的卡片。
		drain:
			for {
				select {
				case call := <-c.queue:
					absorb(buf.add(call))
				default:
					break drain
				}
			}
			absorb(buf.drain())
			if len(pending) > 0 {
				c.send(pending)
			}
			return
		}
	}
}

// send 按事件条数与字节预算把一批事件切成若干次 POST。
func (c *Client) send(evs []event) {
	for len(evs) > 0 {
		n := 0
		size := 0
		for n < len(evs) && n < c.opts.BatchSize {
			item := eventBytes(evs[n])
			if n > 0 && size+item > maxSendBytes {
				break
			}
			size += item
			n++
		}
		c.post(evs[:n])
		evs = evs[n:]
	}
}

func (c *Client) post(evs []event) {
	if len(evs) == 0 {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			// 这一层离 panic 最近，不能只是把异常吞掉：事件已经从数组里出去了，
			// 不计数就等于凭空消失，进程看上去还很健康。
			c.failed.Add(1)
			c.dropped.Add(uint64(len(evs)))
		}
	}()

	payload, err := json.Marshal(batch{Batch: evs})
	if err != nil {
		c.failed.Add(1)
		return
	}
	// 真实字节才是判据：估算函数挡不住 JSON 转义的膨胀（`&` `<` 会翻 6 倍），
	// 一旦越过 Langfuse 的 4.5MB 上限，整批（含卡片）会被 413 带走。
	// 超了就二分重发，宁可多几次请求。
	if len(payload) > maxSendBytes && len(evs) > 1 {
		half := len(evs) / 2
		c.post(evs[:half])
		c.post(evs[half:])
		return
	}
	if len(payload) > maxSendBytes {
		// 单条就超上限：确实发不出去，计入丢弃，让"上报不全"可见。
		c.failed.Add(1)
		c.dropped.Add(1)
		return
	}

	timeout := c.requestTimeout()
	if timeout <= 0 {
		// 退出预算已耗尽：不再发，如实记丢弃。
		c.failed.Add(1)
		c.dropped.Add(uint64(len(evs)))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxIngestionRespBytes))

	if resp.StatusCode >= http.StatusInternalServerError {
		// 5xx 是远端问题：整批计失败，不重试（避免退出路径被拖住）。
		c.failed.Add(1)
		return
	}
	if resp.StatusCode >= http.StatusBadRequest {
		// 只有「内容相关」的状态码才值得二分隔离 —— 它是"这条事件有毒"。
		// 401/403/404/429 是配置或限流问题，二分只会把一次失败放大成上百次请求。
		switch resp.StatusCode {
		case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
			if len(evs) > 1 {
				half := len(evs) / 2
				c.post(evs[:half])
				c.post(evs[half:])
				return
			}
		}
		c.failed.Add(1)
		return
	}

	c.batches.Add(1)
	// Langfuse 的逐事件拒绝不是 4xx，而是 207 + errors[]。只按状态码计成功
	// 会让 Sent/Failed/Dropped 三个自检指标一起说谎 —— 而那是这套观测层
	// 唯一能自查的窗口。
	if rejected := countIngestionErrors(respBody); rejected > 0 {
		bad := uint64(rejected)
		if bad > uint64(len(evs)) {
			bad = uint64(len(evs))
		}
		c.failed.Add(bad)
		c.dropped.Add(bad)
		c.sent.Add(uint64(len(evs)) - bad)
		return
	}
	c.sent.Add(uint64(len(evs)))
}

// countIngestionErrors 解析 ingestion 响应里的逐事件拒绝数。
func countIngestionErrors(body []byte) int {
	if len(body) == 0 {
		return 0
	}
	var report struct {
		Errors []struct {
			ID      string `json:"id"`
			Status  int    `json:"status"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &report); err != nil {
		return 0
	}
	return len(report.Errors)
}
