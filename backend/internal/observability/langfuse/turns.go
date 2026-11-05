package langfuse

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/observability/sessiongroup"
)

// 轮次卡片的边界与默认值。
const (
	// DefaultTurnIdle 是轮次收尾的空闲阈值。
	//
	// 选 5 分钟的依据（最近 300 条真实 trace 实测）：
	//   - 60s  → 38 个轮次里 7 个被切断（52 条 trace 受影响）
	//   - 120s → 2 个被切断
	//   - 300s → 1 个被切断
	//   - 600s → 0 个，但卡片要多等 10 分钟
	// 300s 是「几乎不切断」和「落卡够快」的交点；Langfuse v4 的 events 视图
	// 本身还有约 15 分钟物化延迟，再晚也感知不到。
	DefaultTurnIdle = 5 * time.Minute

	// DefaultMaxTurns 是同时未收尾的轮次上限：超限时强制收尾最老的一轮。
	DefaultMaxTurns = 2048

	// defaultMaxTurnBytes 是未收尾卡片占用的字节预算。
	//
	// 只按张数设上限是不够的：一张卡最多持有 input(32KB) +
	// stepOutput(outputStoreLimit=96KB) + request_body(8KB) ≈ 136KB，
	// 2048 张就是 278MB。这里按字节收口，
	// 与 config 里队列的 128MB 正文字节预算相互独立、互不挤占。
	defaultMaxTurnBytes = 32 << 20

	// maxStartSkew 是「业务开始时刻」与墙钟之间可接受的落后幅度。
	// 超过它且拿不到耗时信息时，按墙钟算，避免 DB 时钟偏差把整站卡片时间戳拖偏。
	maxStartSkew = time.Hour

	// turnInputLimit 是卡片上用户消息的字节上限。
	// 实测用户消息 p50 只有几百字节，但「贴一段代码让我改」这类追问会到几十 KB，
	// 32KB 能完整展示一次追问，又不至于让卡片本身变成正文仓库。
	turnInputLimit = 32 << 10

	// turnOutputLimit 是卡片上 assistant 回答的字节上限。
	//
	// 这一份预算要同时装下「结论」和「过程」两半：
	// 实测一个 40 步的轮次有 15 段正文，单段 p50=107 字节、max=23064 字节，
	// 15 段的上限量级是 345KB —— 64KB 只够放 2.8 段。所以不能按到达顺序先到先得，
	// 那样最后一段（结论）必然被饿死，而 Langfuse 的 trace 列表页只显示
	// output 前约 100 字符，那里必须是结论。分配见 renderOutput。
	turnOutputLimit = 64 << 10

	// outputConclusionLimit 是末段（结论）的保证预算。
	// 取 turnOutputLimit 的一半：剩下的 32KB 留给过程段，
	// 过程段按「就近保留」从后往前分配。
	outputConclusionLimit = 32 << 10

	// outputStoreLimit 是 stepOutput 在内存里允许保留的总字节。
	//
	// 必须封顶：clampCaptured 允许单步 output 到 1MB，一轮 15 步就是 15MB，
	// 乘上未收尾卡片的上限能把进程打穿。96KB 是 turnOutputLimit 的 1.5 倍，
	// 刚好放得下「末段 32KB + 过程 64KB」，收尾渲染时不会因为提前丢弃而失真。
	// 超预算时从最早的步骤开始丢 —— 与 renderOutput 的分配方向一致。
	outputStoreLimit = 96 << 10

	// processHeaderFmt 是结论与过程之间的分隔说明，字节要从总预算里先扣。
	processHeaderFmt = "\n\n——（以下为过程，本轮共 %d 段）——\n\n"

	// turnNoAnswer 是一轮没有任何文本产出时卡片 output 的占位说明。
	// 空 output 会被读成"上报坏了"，写清楚"本轮只有工具调用"才是有效信息。
	turnNoAnswer = "〔本轮无文本回答，仅工具调用/未产出〕"
)

// turnCard 是一轮对话在观测层的卡面。
//
// 它只在轮次收尾时被消费一次，转成 trace-create 事件；卡片内的
// input/output 来自该轮所有步骤的聚合，而不是任意一步的快照。
type turnCard struct {
	traceID string

	userID        int64
	sessionID     string
	sessionSource string
	threadID      string
	turnIndex     int
	title         string

	// input 取该轮第一个非空的最新用户消息：同一轮里每个步骤看到的
	// 「用户最新那句话」都相同，取首个即可，避免后面的步骤把它覆盖成空。
	input          string
	inputTruncated bool

	// turnSigUnknown 表示这一轮认不出轮次签名（抽不到用户消息条数），
	// 只能靠空闲超时切分，卡片有可能把连续几轮串在一起。
	turnSigUnknown bool

	// stepOutput 按轮内步号保存该步的 assistant 正文，收尾时由 renderOutput
	// 统一分配预算拼成卡面。
	//
	// 不在 add 里即时拼接，两个原因：
	//   - 同一步的重试会被重复拼接。sessiongroup 对同一 RequestFingerprint 只把
	//     AttemptIndex +1、StepIndex 不变（sessiongroup.go:269-276），于是
	//     「重试 2 次」在卡上出现 2 份一模一样的回答 —— 正是这次改造要消灭的
	//     重复，只是从 generation 层漏到了卡片层。按步号存，重试自然覆盖。
	//   - 即时拼接是「先到先得」，长轮次里最后一段（结论）必然被饿死。
	stepOutput map[int]string
	// stepBytes 是 stepOutput 里全部正文的字节数，供 size() 与预算淘汰用。
	stepBytes int
	// outputTruncated 标记卡面回答不完整：stepOutput 预算淘汰、渲染截断、
	// 上游正文自身截断，三种来源都并进这一个标记。
	outputTruncated bool
	// outputParts 记录这一轮一共有几段非空输出，不管是否渲染得下。
	// 它按到达次数累加，不受 stepOutput 淘汰影响。
	outputParts int

	startTs time.Time
	lastTs  time.Time

	steps    int
	attempts int
	cost     float64

	// 以下字段都取首步快照：它们描述这一轮的场景，轮内不变。
	endpoint       string
	billingType    string
	billingMode    string
	serviceTier    string
	accountID      int64
	apiKeyID       int64
	groupID        int64
	requestBody    string
	requestBodyCut bool
	historyBytes   int
	// promptTurns 是该轮携带的用户消息条数，只用于给短消息补一句上下文说明。
	promptTurns    int
	promptStatus   string
	promptProtocol string
	promptFallback bool
	lastFinish     string
}

// turnBuffer 累积未收尾的轮次。
//
// 并发模型：只由 client 的单个 worker goroutine 访问，不加锁。
// 这条约束让「会话/轮次的推进天然串行」成为既成事实 —— 业务同步路径
// 依旧只有一次 channel send。
type turnBuffer struct {
	release  string
	idle     time.Duration
	maxCards int
	maxBytes int
	bytes    int
	seq      uint64
	cards    map[string]*turnCard
	// clock 是可注入的时钟。生产用墙钟；测试用它消除 time.Now() 的刻度抖动，
	// 否则「谁最老」这类断言会变成抛硬币。
	clock func() time.Time
}

func (b *turnBuffer) now() time.Time {
	if b.clock != nil {
		return b.clock()
	}
	return time.Now()
}

func newTurnBuffer(release string, idle time.Duration, maxCards int) *turnBuffer {
	if idle <= 0 {
		idle = DefaultTurnIdle
	}
	if maxCards <= 0 {
		maxCards = DefaultMaxTurns
	}
	return &turnBuffer{
		release:  release,
		idle:     idle,
		maxCards: maxCards,
		maxBytes: defaultMaxTurnBytes,
		cards:    make(map[string]*turnCard, 64),
	}
}

// size 是这张卡片当前占用的字节数（可读正文字段 + 固定开销）。
func (c *turnCard) size() int {
	const fixed = 512
	return fixed + len(c.input) + c.stepBytes + len(c.requestBody) + len(c.title)
}

// add 吸收一条调用记录，返回需要立即外发的事件。
//
// 返回的只有 generation-create：卡片（trace-create）要等这一轮真正结束。
// 顺序上 generation 会先于它所属的 trace 到达 Langfuse —— 已用探针验证过
// 这样是安全的：trace 不存在时 generation 被暂存，trace 到达后自动挂上，
// observations 计数与总费用都对得上。
func (b *turnBuffer) add(c Call) []event {
	c = clampCaptured(c)

	// 时间基准分成两个，不能混用：
	//   - startTs 用业务给的开始时刻（卡片按"提问时刻"排序）；
	//   - 空闲判定用「这一步被观测到的墙钟时刻」。
	// 旧写法拿开始时刻判空闲，一步跑 6 分钟（长思考/长流式）就会在入库的瞬间
	// 被判为已过期，卡片在下一步到达前被关掉，一轮被切成几张卡。
	wall := b.now()
	start := c.StartedAt
	if start.IsZero() || start.After(wall) {
		// 业务/DB 时钟超前时把开始时刻钳回墙钟，否则卡片会被钉在未来永不收尾。
		start = wall
	} else if c.Duration <= 0 && wall.Sub(start) > maxStartSkew {
		// 时钟落后到离谱、又没有耗时信息可兜底：按墙钟算，
		// 否则整站卡片时间戳会整体偏移，turn_duration_s 也跟着虚高。
		start = wall
	}
	end := start.Add(c.Duration)
	if c.Duration <= 0 {
		end = start
	}
	if end.After(wall) {
		end = wall
	}
	if end.Before(start) {
		end = start
	}
	c.StartedAt = start
	c.Duration = end.Sub(start)

	grp := sessiongroup.Resolve(sessiongroup.Input{
		ClientSessionID:    c.SessionID,
		UserID:             c.UserID,
		APIKeyID:           c.APIKeyID,
		FirstUserPrompt:    c.FirstUserPrompt,
		UserTurns:          c.UserTurns,
		RequestFingerprint: c.RequestFingerprint,
		TurnIdle:           b.idle,
		Now:                wall,
	})

	card, evicted := b.touch(c, grp, start, wall)
	out := make([]event, 0, len(evicted)+1)
	out = append(out, evicted...)
	out = append(out, generationEvent(c, card.traceID, grp))
	return out
}

// touch 找到（或新建）该轮次的卡片并推进聚合。返回值为「卡面」与「因容量
// 上限被迫提前收尾的旧卡片事件」。
func (b *turnBuffer) touch(c Call, grp sessiongroup.Group, start, wall time.Time) (*turnCard, []event) {
	key := cardKey(c, grp)

	var evicted []event
	card := b.cards[key]
	if card == nil {
		// 张数上限是硬约束；字节预算在卡片填充完之后再核（见函数末尾），
		// 因为一张卡的真实大小取决于它会收到多少正文。
		for len(b.cards) >= b.maxCards {
			next := b.evictOldest()
			if len(next) == 0 {
				break
			}
			evicted = append(evicted, next...)
		}
		b.seq++
		card = &turnCard{
			traceID:        "tr-" + shortHash(key) + "-" + strconv.FormatUint(b.seq, 36),
			userID:         c.UserID,
			sessionID:      grp.SessionID,
			sessionSource:  grp.SessionSource,
			threadID:       grp.ThreadID,
			turnIndex:      grp.TurnIndex,
			title:          grp.Title,
			startTs:        start,
			lastTs:         wall,
			endpoint:       c.Endpoint,
			billingType:    c.BillingType,
			billingMode:    c.BillingMode,
			serviceTier:    c.ServiceTier,
			accountID:      c.AccountID,
			apiKeyID:       c.APIKeyID,
			groupID:        c.GroupID,
			historyBytes:   c.UserPromptHistoryBytes,
			promptTurns:    c.UserTurns,
			promptStatus:   c.PromptStatus,
			promptProtocol: c.PromptProtocol,
			promptFallback: c.PromptFallback,
			turnSigUnknown: c.UserTurns <= 0,
			stepOutput:     make(map[int]string, 8),
		}
		b.cards[key] = card
		// 固定开销在建卡时就计入，否则每张卡都会少算 512 字节，长期累积漂移。
		b.bytes += card.size()
	}

	before := card.size()
	// 乱序到达时卡片仍要从最早那一步开始排。
	if start.Before(card.startTs) {
		card.startTs = start
	}
	if wall.After(card.lastTs) {
		card.lastTs = wall
	}
	// 步数以 sessiongroup 的轮内步号为准：同一步的重试折进 attempt，
	// 不能在这里按调用次数 +1，否则 turn_steps 与 thread_step 自相矛盾。
	if grp.StepIndex > card.steps {
		card.steps = grp.StepIndex
	}
	if grp.AttemptIndex > card.attempts {
		card.attempts = grp.AttemptIndex
	}
	card.cost += c.Cost

	// 用户消息取首个非空值：轮内它恒定，首步拿不到时由后续步骤补上。
	if card.input == "" {
		if user := c.InputUser; user != "" {
			card.input = truncateBytes(user, turnInputLimit)
			card.inputTruncated = c.PromptTruncated || len(user) > turnInputLimit
		}
	}
	// 回答按步号存起来，收尾时统一分配预算（见 renderOutput）。
	// 同一步的重试覆盖而不是追加 —— 这是「重复」在卡片层的最后一道闸。
	if c.Output != "" {
		// 单段先截到结论预算：卡片侧的展示上限就是 outputConclusionLimit，
		// 留更长的只会在淘汰循环之前白占内存。这一步的完整正文在它自己的
		// generation 上，卡片不需要存第二份。
		seg := truncateBytes(c.Output, outputConclusionLimit)
		if len(seg) < len(c.Output) {
			card.outputTruncated = true
		}
		if old, seen := card.stepOutput[grp.StepIndex]; seen {
			// 重试：只换内容，不新增段数，也不让字节数双算。
			card.stepBytes -= len(old)
		} else {
			card.outputParts++
		}
		card.stepOutput[grp.StepIndex] = seg
		card.stepBytes += len(seg)
		// 多段累计仍会超：单段上限 32KB，三段就到 96KB。
		// 淘汰从最早的步骤开始：renderOutput 也优先保留靠近结论的段，
		// 两个方向一致，提前丢弃不会让卡面成色变差。
		for card.stepBytes > outputStoreLimit && len(card.stepOutput) > 1 {
			card.dropOldestStep()
			card.outputTruncated = true
		}
		if c.OutputTruncated {
			card.outputTruncated = true
		}
	}
	if c.FinishReason != "" {
		card.lastFinish = c.FinishReason
	}
	// 原始请求样本只留首步：它是排障用的抽样，每步都存会让卡片没有主次。
	if card.requestBody == "" && c.Input != "" {
		card.requestBody = truncateBytes(c.Input, rawBodyMetaLimit)
		card.requestBodyCut = len(c.Input) > rawBodyMetaLimit
	}

	b.bytes += card.size() - before

	// 字节预算：卡片填充完才知道真实大小，所以在这里核。
	// 必须跳过当前这张：两次 add 可能落在同一时钟刻度上，lastTs 打平后
	// evictOldest 会按 map 迭代顺序随便选，把刚建好、马上还要继续写的卡落掉。
	for b.bytes > b.maxBytes && len(b.cards) > 1 {
		next := b.evictOldestExcept(key)
		if len(next) == 0 {
			break
		}
		evicted = append(evicted, next...)
	}
	return card, evicted
}

// expire 收尾空闲超时的轮次，返回待外发的 trace 事件。
func (b *turnBuffer) expire(now time.Time) []event {
	var out []event
	for key, card := range b.cards {
		if now.Sub(card.lastTs) < b.idle {
			continue
		}
		out = append(out, b.closeCard(card))
		b.bytes -= card.size()
		delete(b.cards, key)
	}
	return out
}

// drain 收尾全部未完成轮次（进程退出路径）。
//
// 代价说清楚：退出时仍在进行中的那一轮，其 generation 已经发出但卡片
// 现在才补上；如果这一批也发不出去，Langfuse 侧就只有一批挂不上 trace 的
// generation（不可见）。观测是 best-effort，这个取舍与「队列满即丢」一致。
func (b *turnBuffer) drain() []event {
	out := make([]event, 0, len(b.cards))
	for key, card := range b.cards {
		out = append(out, b.closeCard(card))
		b.bytes -= card.size()
		delete(b.cards, key)
	}
	b.bytes = 0
	return out
}

// evictOldest 强制收尾最久没有新步骤的一轮。
func (b *turnBuffer) evictOldest() []event { return b.evictOldestExcept("") }

// evictOldestExcept 收尾最久没有新步骤的一轮，跳过 skip 指定的键。
func (b *turnBuffer) evictOldestExcept(skip string) []event {
	var oldestKey string
	var oldest *turnCard
	for key, card := range b.cards {
		if skip != "" && key == skip {
			continue
		}
		if oldest == nil || card.lastTs.Before(oldest.lastTs) {
			oldestKey, oldest = key, card
		}
	}
	if oldest == nil {
		return nil
	}
	b.bytes -= oldest.size()
	delete(b.cards, oldestKey)
	return []event{b.closeCard(oldest)}
}

// pending 返回当前未收尾的轮次数，仅用于测试与自检。
func (b *turnBuffer) pending() int { return len(b.cards) }

// inputMissing 报告这张卡片没有可展示的用户消息。
// 纯工具轮（模型自己发起的后续调用）会出现这种情况，属于正常。
func (c *turnCard) inputMissing() bool { return c.input == "" }

// dropOldestStep 丢掉步号最小的那段输出，返回是否真的丢了。
//
// 只在 stepOutput 超 outputStoreLimit 时调用。段数最多几十，
// 线性找最小键足够，不值得为此维护一个堆。
func (c *turnCard) dropOldestStep() bool {
	if len(c.stepOutput) <= 1 {
		return false
	}
	oldest, found := 0, false
	for step := range c.stepOutput {
		if !found || step < oldest {
			oldest, found = step, true
		}
	}
	if !found {
		return false
	}
	c.stepBytes -= len(c.stepOutput[oldest])
	delete(c.stepOutput, oldest)
	return true
}

// renderOutput 把该轮各步输出拼成卡面正文，预算按「结论优先」分配。
//
// 不能按到达顺序先到先得：实测一轮 15 段正文、每段 20KB 时，64KB 上限只装得下
// 前 3 段，而 agentic 轮的结论永远在最后一段 —— 卡片就变成「只有过程、没有答案」。
// Langfuse 的 trace 列表页只显示 output 前约 100 字符，那里必须是结论。
//
// 分配顺序：
//  1. 末段（结论）全额保留，上限 outputConclusionLimit；
//  2. 剩余预算从后往前给过程段 —— 越靠近结论的越先拿到，越早的过程段越先丢；
//  3. 一段都放不下时只留结论。
//
// 丢掉的段不是消失：展开下面的 generation 还能逐条读到，所以「早的过程段先丢」
// 不损失可查性。
func (c *turnCard) renderOutput() string {
	if len(c.stepOutput) == 0 {
		return ""
	}
	// 按步号升序：map 迭代顺序随机，不排序会让同一张卡每次渲染都不一样。
	steps := make([]int, 0, len(c.stepOutput))
	for step := range c.stepOutput {
		steps = append(steps, step)
	}
	sort.Ints(steps)

	lastSeg := c.stepOutput[steps[len(steps)-1]]
	conclusion := truncateBytes(lastSeg, outputConclusionLimit)
	if len(lastSeg) > outputConclusionLimit {
		c.outputTruncated = true
	}
	if len(steps) == 1 {
		return conclusion
	}

	// 段数用 outputParts 而不是 len(steps)：stepOutput 可能被预算淘汰过，
	// 卡面上要写的是「这一轮原本有几段」，不是「还剩下几段」。
	header := fmt.Sprintf(processHeaderFmt, c.outputParts)
	budget := turnOutputLimit - len(conclusion) - len(header)

	var tail []string
	for i := len(steps) - 2; i >= 0; i-- {
		seg := c.stepOutput[steps[i]]
		sep := 0
		if len(tail) > 0 {
			sep = len("\n\n")
		}
		if len(seg)+sep > budget {
			c.outputTruncated = true
			break
		}
		budget -= len(seg) + sep
		tail = append(tail, seg)
	}
	if len(tail) == 0 {
		return conclusion
	}
	// tail 是倒序收集的，反转回时间正序再拼。
	for i, j := 0, len(tail)-1; i < j; i, j = i+1, j-1 {
		tail[i], tail[j] = tail[j], tail[i]
	}
	return conclusion + header + strings.Join(tail, "\n\n")
}

// shortInputBytes 是"用户只说了几个字"的判定线（"继续"、"好"、"再试一次"）。
// 60 字节约等于 20 个汉字：一句话说完整就超过它了，别再补说明占地方。
const shortInputBytes = 60

// inputWithContext 返回卡片的 input。
//
// agentic 会话里用户经常只说"继续"两个字，卡片上就只剩这一点内容，
// 读起来像采集不全。这里给短消息补一行上下文规模说明：用户能看出
// "不是抓漏了，是我这轮就说了这几个字"，也能判断携带了多少历史。
func (c *turnCard) inputWithContext() string {
	if len(c.input) > shortInputBytes || c.historyBytes <= 0 {
		return c.input
	}
	turns := c.promptTurns
	if turns <= 0 {
		turns = 1
	}
	return c.input + "\n\n——（本轮用户消息；请求携带 " +
		strconv.Itoa(turns) + " 条历史、" +
		formatKB(c.historyBytes) + " 上下文，系统提示与工具定义不在卡片上）"
}

// formatKB 把字节数写成人读的规模；不足 1KB 时保留一位小数，避免出现"0 KB"。
func formatKB(n int) string {
	if n >= 1<<20 {
		return strconv.FormatFloat(float64(n)/(1<<20), 'f', 1, 64) + " MB"
	}
	if n >= 1<<10 {
		return strconv.Itoa(n>>10) + " KB"
	}
	if n <= 0 {
		return "0 B"
	}
	return strconv.Itoa(n) + " B"
}

// cardKey 是卡片的聚合键。
//
// 优先用归组算出的 TurnID（会话 + 线程 + 轮次签名），这样并发的多个子代理
// 各自成卡、互不干扰。轮次签名不可信时**不能**用 TurnID —— 它退化成"全会话
// 一个桶"，会把连续 N 轮并成一张内容错乱的大卡；此时退到请求级键，
// 效果等于回到「一次上游调用一张」，但至少每张卡是干净的。
func cardKey(c Call, grp sessiongroup.Group) string {
	if grp.TurnKnown && grp.TurnID != "" {
		return grp.TurnID
	}
	if c.RequestFingerprint != "" {
		return "fp|" + c.RequestFingerprint
	}
	if c.RequestID != "" {
		return "rid|" + c.RequestID
	}
	return "anon|" + newID()
}

// closeCard 把一张卡面转成 trace-create 事件。
func (b *turnBuffer) closeCard(card *turnCard) event {
	// 先渲染正文再建 metadata：renderOutput 会补齐 outputTruncated
	// （渲染时的预算截断只有它知道），而 cardMetadata 要读这个标记。
	out := card.renderOutput()
	body := map[string]any{
		"id":        card.traceID,
		"name":      turnDisplayName(card),
		"timestamp": formatTime(card.startTs),
		"metadata":  cardMetadata(card),
	}
	// 会话视图里每张卡片只显示 trace 名，靠 tag 才能按线程过滤。
	if card.threadID != "" {
		body["tags"] = []string{"thread:" + card.threadID}
	}
	if card.userID != 0 {
		body["userId"] = strconv.FormatInt(card.userID, 10)
	}
	if card.sessionID != "" {
		body["sessionId"] = card.sessionID
	}
	if b.release != "" {
		body["release"] = b.release
	}
	if card.input != "" {
		body["input"] = card.inputWithContext()
	}
	if out != "" {
		body["output"] = out
	} else {
		body["output"] = turnNoAnswer
	}
	return event{
		ID:        "ev-" + newID() + "-t",
		Type:      eventTypeTraceCreate,
		Timestamp: formatTime(card.startTs),
		Body:      body,
	}
}

// generationEvent 把一条调用记录转成 generation-create 事件。
func generationEvent(c Call, traceID string, grp sessiongroup.Group) event {
	id := newID()
	return event{
		ID:        "ev-" + id + "-g",
		Type:      eventTypeGenerationCreate,
		Timestamp: formatTime(c.StartedAt),
		Body:      generationBody(c, traceID, "gn-"+id, grp),
	}
}

// turnDisplayName 让会话视图里的每张卡片可区分。
//
// 一轮一张卡之后，卡片数量从「每次上游调用」降到「每次用户提问」，
// 名字只需要带上线程 + 轮次；具体问的是什么由 input 直接展示。
func turnDisplayName(card *turnCard) string {
	name := traceName
	if short := shortThreadID(card.threadID); short != "" {
		name += " · " + short
	}
	if card.turnIndex > 0 {
		name += " · t" + strconv.Itoa(card.turnIndex)
	}
	return name
}

func shortThreadID(threadID string) string {
	trimmed := threadID
	if len(trimmed) > 3 && trimmed[:3] == "th-" {
		trimmed = trimmed[3:]
	}
	// 6 位十六进制 = 1600 万种取值：会话里有几十条线程时，4 位（65536）
	// 按生日界已经会出现肉眼可见的重名。
	const keep = 6
	if len(trimmed) <= keep {
		return trimmed
	}
	return trimmed[:keep]
}

// cardMetadata 是卡片级 metadata：描述这一轮整体。
func cardMetadata(card *turnCard) map[string]any {
	meta := map[string]any{
		"cost_source": costSourceActual,
		"turn_steps":  card.steps,
	}
	if card.sessionSource != "" && card.sessionSource != sessiongroup.SourceNone {
		meta["session_source"] = card.sessionSource
	}
	if card.title != "" {
		meta["session_title"] = card.title
	}
	if card.threadID != "" {
		meta["thread_id"] = card.threadID
	}
	if card.turnIndex > 0 {
		meta["turn_index"] = card.turnIndex
	}
	if card.attempts > 0 {
		meta["turn_attempts"] = card.attempts
	}
	if duration := card.lastTs.Sub(card.startTs); duration > 0 {
		meta["turn_duration_s"] = int(duration.Seconds())
	}
	if card.cost != 0 {
		meta["turn_cost"] = clampFloat(card.cost)
	}
	if card.promptStatus != "" {
		meta["user_prompt_status"] = card.promptStatus
	}
	if card.promptProtocol != "" {
		meta["user_prompt_protocol"] = card.promptProtocol
	}
	if card.promptFallback {
		meta["user_prompt_fallback"] = true
	}
	if card.historyBytes > 0 {
		meta["user_prompt_history_bytes"] = card.historyBytes
	}
	if card.inputMissing() {
		meta["turn_input_missing"] = true
	}
	if card.turnSigUnknown {
		// 认不出轮次签名时，卡片可能把连续几轮串在一起；标出来便于判读。
		meta["turn_signature_fallback"] = true
	}
	if card.inputTruncated {
		meta["turn_input_truncated"] = true
	}
	if card.outputParts > 0 {
		// 这一轮的输出由几段拼成：段数远大于 1 说明答案分散在多步里，
		// 只读卡片容易漏掉过程，需要展开下面的步骤。
		meta["turn_output_parts"] = card.outputParts
	}
	if card.outputTruncated {
		meta["turn_output_truncated"] = true
	}
	if card.lastFinish != "" {
		meta["turn_finish_reason"] = card.lastFinish
	}
	if card.requestBody != "" {
		meta["request_body"] = card.requestBody
		// 只在真截断时写：这个键之前无条件写 false，与其他标记（只在 true 时出现）
		// 风格不一致，查询侧得先判存在再判值，容易写成 two-step。
		if card.requestBodyCut {
			meta["request_body_truncated"] = true
		}
	}
	if card.endpoint != "" {
		meta["endpoint"] = card.endpoint
	}
	if card.billingType != "" {
		meta["billing_type"] = card.billingType
	}
	if card.billingMode != "" {
		meta["billing_mode"] = card.billingMode
	}
	if card.serviceTier != "" {
		meta["service_tier"] = card.serviceTier
	}
	if card.accountID != 0 {
		meta["account_id"] = strconv.FormatInt(card.accountID, 10)
	}
	if card.apiKeyID != 0 {
		meta["api_key_id"] = strconv.FormatInt(card.apiKeyID, 10)
	}
	if card.groupID != 0 {
		meta["group_id"] = strconv.FormatInt(card.groupID, 10)
	}
	return meta
}
