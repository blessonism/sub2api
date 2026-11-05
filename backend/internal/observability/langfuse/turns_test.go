package langfuse

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/observability/sessiongroup"
)

func newTestBuffer(t *testing.T) *turnBuffer {
	t.Helper()
	sessiongroup.Reset()
	return newTurnBuffer("rel-test", DefaultTurnIdle, DefaultMaxTurns)
}

// stepCall 造一条「某一轮里的某一步」调用记录。
func stepCall(sessionID, firstUser string, turns int, fp string, at time.Time) Call {
	return Call{
		UserID:             120,
		APIKeyID:           7,
		SessionID:          sessionID,
		FirstUserPrompt:    firstUser,
		RequestFingerprint: fp,
		UserTurns:          turns,
		Model:              "gpt-6-astra",
		Endpoint:           "/v1/responses",
		Status:             "ok",
		StartedAt:          at,
		RequestID:          "req-" + fp,
		Input:              "raw-body-" + fp,
		InputUser:          firstUser,
	}
}

func eventsOfType(evs []event, typ string) []event {
	out := make([]event, 0, len(evs))
	for _, e := range evs {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

func metaOfEvent(t *testing.T, e event) map[string]any {
	t.Helper()
	meta, ok := e.Body["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata 缺失或类型不对：%T", e.Body["metadata"])
	}
	return meta
}

// metaInt 读 metadata 里的数值：同包内是 int，经 JSON 往返后是 float64。
func metaInt(t *testing.T, e event, key string) int {
	t.Helper()
	switch v := metaOfEvent(t, e)[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	t.Fatalf("metadata[%q] 不是数值：%v", key, metaOfEvent(t, e)[key])
	return 0
}

// TestTurnProducesOneCardForManySteps 是本设计的核心断言：
// 一轮对话里的 N 次上游调用必须收敛成 1 张卡片 + N 条步骤。
func TestTurnProducesOneCardForManySteps(t *testing.T) {
	b := newTestBuffer(t)
	// 用相对时间：绝对日期一旦落在墙钟之前超过 maxStartSkew，就会被钳成"现在"，
	// 断言随之变成时钟的奴隶。
	base := time.Now().Add(-30 * time.Minute).UTC().Truncate(time.Second)

	var immediate []event
	for i := 0; i < 5; i++ {
		c := stepCall("sess-a", "把观测改成一轮一张卡", 3, fmt.Sprintf("fp-%d", i), base.Add(time.Duration(i)*30*time.Second))
		c.Output = fmt.Sprintf("第 %d 步的说明", i)
		immediate = append(immediate, b.add(c)...)
	}

	// 轮次还开着：只允许出现 generation，卡片必须等到收尾。
	if got := len(eventsOfType(immediate, eventTypeTraceCreate)); got != 0 {
		t.Fatalf("轮次未结束就落了 %d 张卡", got)
	}
	gens := eventsOfType(immediate, eventTypeGenerationCreate)
	if len(gens) != 5 {
		t.Fatalf("generation 数 = %d, want 5", len(gens))
	}
	if b.pending() != 1 {
		t.Fatalf("未收尾轮次数 = %d, want 1", b.pending())
	}

	cards := b.drain()
	if len(cards) != 1 {
		t.Fatalf("收尾产出 %d 张卡，want 1", len(cards))
	}
	card := cards[0]
	if card.Type != eventTypeTraceCreate {
		t.Fatalf("收尾事件类型 = %q", card.Type)
	}
	// 5 条 generation 必须都挂在这张卡上。
	for _, g := range gens {
		if g.Body["traceId"] != card.Body["id"] {
			t.Fatalf("generation.traceId = %v, want %v", g.Body["traceId"], card.Body["id"])
		}
	}
	// 时间戳取首步，卡片才会按"提问时刻"排序。
	if want := base.Format(tsLayout); card.Body["timestamp"] != want {
		t.Fatalf("卡片时间戳 = %v, want %v（应取该轮首步时间）", card.Body["timestamp"], want)
	}
	if got := metaInt(t, card, "turn_steps"); got != 5 {
		t.Fatalf("turn_steps = %d, want 5", got)
	}
	if b.pending() != 0 {
		t.Fatalf("收尾后仍残留 %d 张卡", b.pending())
	}
}

// TestCardTakesLastNonEmptyAnswer：轮次常常以「纯工具调用」的一步收尾，
// 回答必须取最后一条**非空**文本，否则一半卡片没有 output。
func TestCardTakesLastNonEmptyAnswer(t *testing.T) {
	b := newTestBuffer(t)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

	steps := []string{"先看一下仓库结构", "", "改完再跑测试", ""}
	for i, out := range steps {
		c := stepCall("sess-b", "修一下观测", 1, fmt.Sprintf("fp-%d", i), base.Add(time.Duration(i)*time.Minute))
		c.Output = out
		b.add(c)
	}

	card := b.drain()[0]
	if card.Body["output"] != "改完再跑测试" {
		t.Fatalf("卡片 output = %v，应取最后一条非空回答", card.Body["output"])
	}
	if card.Body["input"] != "修一下观测" {
		t.Fatalf("卡片 input = %v", card.Body["input"])
	}
}

// TestCardMarksTurnWithoutAnswer：整轮真的一句正文都没有时，
// 卡片仍要落，但要写清楚"无文本回答"，不能留空让人以为是上报坏了。
func TestCardMarksTurnWithoutAnswer(t *testing.T) {
	b := newTestBuffer(t)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		b.add(stepCall("sess-c", "只是跑工具", 1, fmt.Sprintf("fp-%d", i), base.Add(time.Duration(i)*time.Minute)))
	}
	card := b.drain()[0]
	if card.Body["output"] != turnNoAnswer {
		t.Fatalf("空回答卡片 output = %v", card.Body["output"])
	}
}

// TestTurnAdvanceMakesSecondCard：用户再次开口（条数 +1）必须另起一张卡。
func TestTurnAdvanceMakesSecondCard(t *testing.T) {
	b := newTestBuffer(t)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

	b.add(stepCall("sess-d", "第一问", 1, "fp-1a", base))
	b.add(stepCall("sess-d", "第一问", 1, "fp-1b", base.Add(time.Minute)))
	b.add(stepCall("sess-d", "第一问", 2, "fp-2a", base.Add(2*time.Minute)))
	b.add(stepCall("sess-d", "第一问", 2, "fp-2b", base.Add(3*time.Minute)))

	if b.pending() != 2 {
		t.Fatalf("未收尾轮次数 = %d, want 2", b.pending())
	}
	cards := b.drain()
	if len(cards) != 2 {
		t.Fatalf("卡片数 = %d, want 2", len(cards))
	}
	// 卡片 id 必须不同，否则后一张会覆盖前一张。
	if cards[0].Body["id"] == cards[1].Body["id"] {
		t.Fatal("两张卡的 trace id 相同")
	}
	for _, c := range cards {
		if got := metaInt(t, c, "turn_steps"); got != 2 {
			t.Fatalf("卡片步数 = %d, want 2", got)
		}
	}
}

// TestInterleavedSubagentsStaySeparate 覆盖实测里最关键的一个坑：
// 同一线程里两个并发子代理的轮次条数会在 3 和 14 之间来回跳，
// 「条数前进就关上一轮」会把卡片切碎。这里要求两个流各自成卡、互不串步。
func TestInterleavedSubagentsStaySeparate(t *testing.T) {
	b := newTestBuffer(t)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

	seq := []int{3, 14, 3, 14, 3, 14}
	for i, turns := range seq {
		b.add(stepCall("sess-e", "并行子代理", turns, fmt.Sprintf("fp-%d", i), base.Add(time.Duration(i)*time.Minute)))
	}

	cards := b.drain()
	if len(cards) != 2 {
		t.Fatalf("卡片数 = %d, want 2（条数 3 与 14 各一张）", len(cards))
	}
	// 条数 3 的流有 3 步、条数 14 的流也有 3 步：两张卡都必须是 3 步，
	// 步数被串成 6 就是两路合并了。
	for _, c := range cards {
		if got := metaInt(t, c, "turn_steps"); got != 3 {
			t.Fatalf("卡片步数 = %d, want 3（两路互不串步）", got)
		}
	}
	// 轮次序号必须一个是 3、一个是 14。
	seen := map[int]bool{}
	for _, c := range cards {
		seen[metaInt(t, c, "turn_index")] = true
	}
	if !seen[3] || !seen[14] {
		t.Fatalf("卡片的 turn_index = %v，want {3,14}", seen)
	}
}

// TestIdleExpiryClosesTurn：没有显式结束信号时，只有空闲超时能收尾。
func TestIdleExpiryClosesTurn(t *testing.T) {
	sessiongroup.Reset()
	b := newTurnBuffer("rel-test", time.Minute, DefaultMaxTurns)

	b.add(stepCall("sess-f", "空闲收尾", 1, "fp-1", time.Now()))

	// 还没到阈值：不能收尾。
	if evs := b.expire(time.Now().Add(30 * time.Second)); len(evs) != 0 {
		t.Fatalf("未到阈值就收尾了 %d 张卡", len(evs))
	}
	// 到点：收尾，且只收一次。
	evs := b.expire(time.Now().Add(61 * time.Second))
	if len(evs) != 1 {
		t.Fatalf("空闲超时应收尾 1 张卡，实际 %d", len(evs))
	}
	if b.pending() != 0 {
		t.Fatalf("收尾后仍残留 %d 张卡", b.pending())
	}
	if again := b.expire(time.Now().Add(10 * time.Minute)); len(again) != 0 {
		t.Fatalf("同一张卡被重复收尾 %d 次", len(again))
	}
}

// TestLongStepIsNotExpiredOnArrival：一步跑得比空闲阈值还久时，
// 卡片不能在入库那一瞬间就被判过期（旧实现按请求开始时刻判空闲，会中这个坑）。
func TestLongStepIsNotExpiredOnArrival(t *testing.T) {
	b := newTurnBuffer("rel-test", 2*time.Minute, DefaultMaxTurns)
	// 开始于 10 分钟前、跑了 9 分钟：结束时刻离现在只有 1 分钟。
	started := time.Now().Add(-10 * time.Minute)
	c := stepCall("sess-long", "长步骤", 1, "fp-1", started)
	c.Duration = 9 * time.Minute
	b.add(c)

	if evs := b.expire(time.Now()); len(evs) != 0 {
		t.Fatalf("长步骤刚入库就被收尾了 %d 张卡", len(evs))
	}
	if evs := b.expire(time.Now().Add(3 * time.Minute)); len(evs) != 1 {
		t.Fatalf("真正空闲超时后应收尾 1 张卡，实际 %d", len(evs))
	}
}

// TestExpiredTurnReopensAsNewCard：一轮被空闲切掉后，同一条数再来必须换新卡，
// 不能往已经发出去的 trace id 上追加（Langfuse 的 trace-create 不更新已有 trace）。
func TestExpiredTurnReopensAsNewCard(t *testing.T) {
	sessiongroup.Reset()
	b := newTurnBuffer("rel-test", time.Minute, DefaultMaxTurns)

	b.add(stepCall("sess-g", "切了再开", 1, "fp-1", time.Now()))
	first := b.expire(time.Now().Add(2 * time.Minute))
	if len(first) != 1 {
		t.Fatalf("首轮未收尾")
	}
	b.add(stepCall("sess-g", "切了再开", 1, "fp-2", time.Now()))
	second := b.drain()
	if len(second) != 1 {
		t.Fatalf("第二轮卡片数 = %d", len(second))
	}
	if first[0].Body["id"] == second[0].Body["id"] {
		t.Fatal("切分后的两张卡复用了同一个 trace id")
	}
}

// TestGenerationCarriesStepInput：步骤面板不能空着。
// 有工具结果就展示工具结果（每步不同，有信息量），
// 没有就退回该轮的用户消息，而不是留一个 null 让 UI 渲染成 undefined。
func TestGenerationCarriesStepInput(t *testing.T) {
	b := newTestBuffer(t)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

	c := stepCall("sess-h", "同一句话", 1, "fp-1", base)
	c.StepInput = "README.md: 120 行，含安装说明"
	c.StepInputKind = "tool_result"
	c.Output = "这一步的产出"
	evs := b.add(c)
	if len(evs) != 1 || evs[0].Type != eventTypeGenerationCreate {
		t.Fatalf("期望恰好 1 条 generation，实际 %d", len(evs))
	}
	if got := evs[0].Body["input"]; got != "README.md: 120 行，含安装说明" {
		t.Fatalf("generation.input = %v，应优先展示工具结果", got)
	}
	if evs[0].Body["output"] != "这一步的产出" {
		t.Fatalf("generation 必须保留本步输出：%v", evs[0].Body["output"])
	}
	if metaOfEvent(t, evs[0])["step_input_kind"] != "tool_result" {
		t.Fatalf("step_input_kind = %v", metaOfEvent(t, evs[0])["step_input_kind"])
	}
}

// TestGenerationInputFallsBackToUserPrompt：首步没有工具结果，
// 退回该轮的用户消息，而不是空着。
func TestGenerationInputFallsBackToUserPrompt(t *testing.T) {
	b := newTestBuffer(t)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

	evs := b.add(stepCall("sess-h2", "把观测改成一轮一张卡", 1, "fp-1", base))
	if got := evs[0].Body["input"]; got != "把观测改成一轮一张卡" {
		t.Fatalf("generation.input = %v，应退回用户消息", got)
	}
}

// TestGenerationInputIsCapped：工具结果动辄几十 KB，步骤面板只需要看清楚
// "这一步拿到了什么"，按 4KB 收口。
func TestGenerationInputIsCapped(t *testing.T) {
	b := newTestBuffer(t)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

	c := stepCall("sess-h3", "看一个很大的文件", 1, "fp-1", base)
	c.StepInput = strings.Repeat("x", 64<<10)
	evs := b.add(c)
	got, _ := evs[0].Body["input"].(string)
	if len(got) > stepInputLimit {
		t.Fatalf("generation.input = %d 字节，超过 %d 的上限", len(got), stepInputLimit)
	}
}

// TestStepMetadataCarriesTurnPosition：轮内定位靠 thread_id + turn_index + thread_step。
func TestStepMetadataCarriesTurnPosition(t *testing.T) {
	b := newTestBuffer(t)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

	var gens []event
	gens = append(gens, b.add(stepCall("sess-i", "定位", 4, "fp-a", base))...)
	gens = append(gens, b.add(stepCall("sess-i", "定位", 4, "fp-a", base.Add(30*time.Second)))...)
	gens = append(gens, b.add(stepCall("sess-i", "定位", 4, "fp-b", base.Add(time.Minute)))...)
	b.drain()

	if len(gens) != 3 {
		t.Fatalf("generation 数 = %d", len(gens))
	}
	for i, g := range gens {
		if g.Type != eventTypeGenerationCreate {
			t.Fatalf("轮内不该出现 trace 事件：%s", g.Type)
		}
		if got := metaInt(t, g, "turn_index"); got != 4 {
			t.Fatalf("generation[%d].turn_index = %d, want 4", i, got)
		}
	}
	// 第 1、2 次是同一步的重试：步号不动、尝试数 +1。
	if got := metaInt(t, gens[0], "thread_step"); got != 1 {
		t.Fatalf("gens[0].thread_step = %d, want 1", got)
	}
	if got := metaInt(t, gens[0], "step_attempt"); got != 1 {
		t.Fatalf("gens[0].step_attempt = %d, want 1", got)
	}
	if got := metaInt(t, gens[1], "thread_step"); got != 1 {
		t.Fatalf("gens[1].thread_step = %d, want 1（重复请求体不该开新步）", got)
	}
	if got := metaInt(t, gens[1], "step_attempt"); got != 2 {
		t.Fatalf("gens[1].step_attempt = %d, want 2", got)
	}
	if got := metaInt(t, gens[2], "thread_step"); got != 2 {
		t.Fatalf("gens[2].thread_step = %d, want 2（换了请求体才是下一步）", got)
	}
}

// TestMaxCardsEvictsOldest：容量保护必须强制收尾最老的一轮，而不是无限增长。
//
// "谁最老"由墙钟判定，所以这里注入一个单调时钟 —— 否则三次 add 落在同一
// 时钟刻度上时，断言会变成抛硬币。
func TestMaxCardsEvictsOldest(t *testing.T) {
	sessiongroup.Reset()
	b := newTurnBuffer("rel-test", DefaultTurnIdle, 2)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	tick := 0
	b.clock = func() time.Time {
		tick++
		return base.Add(time.Duration(tick) * time.Second)
	}

	b.add(stepCall("sess-j", "第一问", 1, "fp-a", base))
	b.add(stepCall("sess-j", "第二问", 2, "fp-b", base.Add(time.Minute)))
	if b.pending() != 2 {
		t.Fatalf("pending = %d, want 2", b.pending())
	}
	// 第三轮出现时，必须挤掉最老的一轮。
	evs := b.add(stepCall("sess-j", "第三问", 3, "fp-c", base.Add(2*time.Minute)))
	traces := eventsOfType(evs, eventTypeTraceCreate)
	if len(traces) != 1 {
		t.Fatalf("容量超限应强制收尾 1 张卡，实际 %d", len(traces))
	}
	if got := metaInt(t, traces[0], "turn_index"); got != 1 {
		t.Fatalf("被挤掉的应是最早的轮次，实际 turn_index = %d", got)
	}
	if b.pending() != 2 {
		t.Fatalf("pending = %d, want 2", b.pending())
	}
}

// TestBudgetEvictionNeverDropsCurrentCard 覆盖"两次 add 落在同一时钟刻度"：
// evictOldest 用严格小于比较，打平时会按 map 迭代顺序随便选；
// 若选中正在填充的这张，一轮就会裂成两条 trace。
func TestBudgetEvictionNeverDropsCurrentCard(t *testing.T) {
	sessiongroup.Reset()
	b := newTurnBuffer("rel-test", DefaultTurnIdle, 10000)
	b.maxBytes = 8 << 10 // 一张大卡就会触顶
	frozen := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	b.clock = func() time.Time { return frozen } // 所有人 lastTs 全打平

	base := frozen
	for i := 0; i < 6; i++ {
		c := stepCall(fmt.Sprintf("sess-freeze-%d", i), "打平时钟", 1,
			fmt.Sprintf("fp-%d", i), base)
		c.Output = strings.Repeat("答", 4<<10)
		b.add(c)
	}
	// 当前卡（最后一个 key）必须还在缓冲里。
	live := map[string]bool{}
	for _, card := range b.cards {
		live[card.traceID] = true
	}
	if len(live) == 0 {
		t.Fatal("打平时钟下把卡片全挤空了")
	}
	for _, card := range b.cards {
		if card.steps == 0 {
			t.Fatalf("缓冲里混入了从未填充的卡片 %s", card.traceID)
		}
	}
}

// TestCardCarriesSessionAndThreadKeys：会话聚合与线程过滤依赖这两个字段。
func TestCardCarriesSessionAndThreadKeys(t *testing.T) {
	b := newTestBuffer(t)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	b.add(stepCall("01a1ab68-7e7d", "会话聚合", 1, "fp-a", base))

	card := b.drain()[0]
	if card.Body["sessionId"] != "01a1ab68-7e7d" {
		t.Fatalf("sessionId = %v", card.Body["sessionId"])
	}
	if card.Body["userId"] != "120" {
		t.Fatalf("userId = %v", card.Body["userId"])
	}
	tags, ok := card.Body["tags"].([]string)
	if !ok || len(tags) != 1 || !strings.HasPrefix(tags[0], "thread:th-") {
		t.Fatalf("tags = %v", card.Body["tags"])
	}
	name, _ := card.Body["name"].(string)
	if !strings.HasPrefix(name, traceName) || !strings.Contains(name, " · t1") {
		t.Fatalf("卡片名 = %q", name)
	}
}

// TestUngroupableCallStillGetsCard：既无会话头也无用户消息时不能把记录丢掉。
func TestUngroupableCallStillGetsCard(t *testing.T) {
	b := newTestBuffer(t)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	c := Call{UserID: 9, Model: "m", Status: "ok", StartedAt: base, RequestID: "req-1"}
	evs := b.add(c)
	if len(eventsOfType(evs, eventTypeGenerationCreate)) != 1 {
		t.Fatal("无归组信息时仍要产出 generation")
	}
	cards := b.drain()
	if len(cards) != 1 {
		t.Fatalf("无归组信息时仍要落 1 张卡，实际 %d", len(cards))
	}
	if _, ok := cards[0].Body["sessionId"]; ok {
		t.Fatalf("无会话信息不该落 sessionId")
	}
}

// TestCardAccumulatesTurnCost：一轮的花费要能一眼看出来。
func TestCardAccumulatesTurnCost(t *testing.T) {
	b := newTestBuffer(t)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		c := stepCall("sess-k", "算钱", 1, fmt.Sprintf("fp-%d", i), base.Add(time.Duration(i)*time.Second))
		c.Cost = 0.125
		b.add(c)
	}
	card := b.drain()[0]
	if got, _ := metaOfEvent(t, card)["turn_cost"].(float64); got != 0.5 {
		t.Fatalf("turn_cost = %v, want 0.5", metaOfEvent(t, card)["turn_cost"])
	}
}

// TestRetryDoesNotInflateTurnSteps：同一步的重投不能把卡片步数算多，
// 否则 turn_steps 与 generation 上的 thread_step 会自相矛盾。
func TestRetryDoesNotInflateTurnSteps(t *testing.T) {
	b := newTestBuffer(t)
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

	b.add(stepCall("sess-r", "重试", 1, "fp-same", base))
	b.add(stepCall("sess-r", "重试", 1, "fp-same", base.Add(10*time.Second)))
	b.add(stepCall("sess-r", "重试", 1, "fp-same", base.Add(20*time.Second)))

	card := b.drain()[0]
	if got := metaInt(t, card, "turn_steps"); got != 1 {
		t.Fatalf("turn_steps = %d, want 1（三次重投仍是同一步）", got)
	}
	if got := metaInt(t, card, "turn_attempts"); got != 3 {
		t.Fatalf("turn_attempts = %d, want 3", got)
	}
}

// TestTurnBufferStaysWithinByteBudget：只按张数设上限不够，
// 每张卡最多 100KB，2048 张就是 200MB+；字节预算必须真的生效。
func TestTurnBufferStaysWithinByteBudget(t *testing.T) {
	sessiongroup.Reset()
	b := newTurnBuffer("rel-test", DefaultTurnIdle, 10000)
	b.maxBytes = 256 << 10 // 压到很小，方便触发

	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	const total = 40
	var emitted []event
	for i := 0; i < total; i++ {
		// 每轮一个独立会话头，保证造出 40 张互不相同的卡（不能靠 a..z 循环，
		// 那样只有 26 个键，预算压力比意图低一截）。
		c := stepCall(fmt.Sprintf("sess-budget-%d", i), "占预算", 1,
			fmt.Sprintf("fp-%d", i), base.Add(time.Duration(i)*time.Second))
		c.Output = strings.Repeat("答", 30<<10) // 每张卡约 30KB
		emitted = append(emitted, b.add(c)...)
	}

	if b.bytes > b.maxBytes {
		t.Fatalf("未收尾卡片占用 %d 字节，超过预算 %d", b.bytes, b.maxBytes)
	}
	if b.pending() == 0 {
		t.Fatal("预算不该把卡片全部清空到零")
	}
	// 被挤掉的卡片必须真的落成了 trace 事件，不能凭空消失。
	traces := eventsOfType(emitted, eventTypeTraceCreate)
	if len(traces) == 0 {
		t.Fatal("字节预算从未触发淘汰：40 张 30KB 的卡塞进 256KB 预算必然要挤掉一些")
	}
	if len(traces)+b.pending() != total {
		t.Fatalf("落卡 %d + 未收尾 %d != %d，有卡片丢了", len(traces), b.pending(), total)
	}
	// 同一张卡不能被落两次。
	seen := make(map[string]bool, len(traces))
	for _, tr := range traces {
		id, _ := tr.Body["id"].(string)
		if seen[id] {
			t.Fatalf("卡片 %s 被重复落卡", id)
		}
		seen[id] = true
	}
	for _, card := range b.cards {
		if seen[card.traceID] {
			t.Fatalf("卡片 %s 既已落卡又还在缓冲里", card.traceID)
		}
	}
}
