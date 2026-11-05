//go:build unit

package sessiongroup

import (
	"strings"
	"testing"
	"time"
)

func baseNow() time.Time { return time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC) }

func TestClientSessionIDWins(t *testing.T) {
	reg := NewRegistry(0, 0, 0)
	grp := reg.Resolve(Input{
		ClientSessionID: "01a1ab68-7e7d",
		UserID:          120,
		APIKeyID:        7,
		FirstUserPrompt: "把会话聚合补上",
		Now:             baseNow(),
	})
	if grp.SessionID != "01a1ab68-7e7d" {
		t.Fatalf("sessionID = %q", grp.SessionID)
	}
	if grp.SessionSource != SourceClient {
		t.Fatalf("source = %q", grp.SessionSource)
	}
	if grp.ThreadID == "" || grp.StepIndex != 1 || grp.AttemptIndex != 1 {
		t.Fatalf("group = %+v", grp)
	}
}

func TestAutoSessionIDIsStableAcrossTurns(t *testing.T) {
	reg := NewRegistry(0, 0, 0)
	now := baseNow()
	first := reg.Resolve(Input{UserID: 9, APIKeyID: 3, FirstUserPrompt: "重构网关", RequestFingerprint: "a", Now: now})
	second := reg.Resolve(Input{UserID: 9, APIKeyID: 3, FirstUserPrompt: "重构网关", RequestFingerprint: "b", Now: now.Add(time.Minute)})
	if first.SessionSource != SourceAuto {
		t.Fatalf("source = %q", first.SessionSource)
	}
	if !strings.HasPrefix(first.SessionID, "auto-") {
		t.Fatalf("sessionID = %q", first.SessionID)
	}
	if first.SessionID != second.SessionID {
		t.Fatalf("同一锚点必须复用同一会话 id：%q vs %q", first.SessionID, second.SessionID)
	}
	if second.StepIndex != 2 {
		t.Fatalf("step = %d, want 2（换了请求体就是新的一步）", second.StepIndex)
	}
}

func TestDifferentFirstPromptSplitsThreads(t *testing.T) {
	reg := NewRegistry(0, 0, 0)
	now := baseNow()
	a := reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, FirstUserPrompt: "线程A 的起点", Now: now})
	b := reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, FirstUserPrompt: "线程B 的起点", Now: now})
	if a.SessionID != b.SessionID {
		t.Fatal("同一客户端会话头必须落到同一会话")
	}
	if a.ThreadID == b.ThreadID {
		t.Fatal("不同起点消息必须切成不同线程（这正是线上 3/8 交替的两条流）")
	}
	if b.StepIndex != 1 {
		t.Fatalf("新线程 step = %d, want 1", b.StepIndex)
	}
}

func TestSameFingerprintCountsAsAttempt(t *testing.T) {
	reg := NewRegistry(0, 0, 0)
	now := baseNow()
	first := reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, FirstUserPrompt: "x", RequestFingerprint: "same", Now: now})
	retry := reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, FirstUserPrompt: "x", RequestFingerprint: "same", Now: now.Add(30 * time.Second)})
	if retry.StepIndex != 1 || retry.AttemptIndex != 2 {
		t.Fatalf("重试应折叠成同一步的 attempt：step=%d attempt=%d", retry.StepIndex, retry.AttemptIndex)
	}
	if first.AttemptIndex != 1 {
		t.Fatalf("首次 attempt = %d", first.AttemptIndex)
	}
}

func TestSameFingerprintAfterWindowIsNewStep(t *testing.T) {
	reg := NewRegistry(0, 0, 0)
	now := baseNow()
	reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, FirstUserPrompt: "x", RequestFingerprint: "same", Now: now})
	later := reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, FirstUserPrompt: "x", RequestFingerprint: "same", Now: now.Add(attemptWindow + time.Minute)})
	if later.StepIndex != 2 || later.AttemptIndex != 1 {
		t.Fatalf("超出尝试窗口的相同请求体应算新一步：step=%d attempt=%d", later.StepIndex, later.AttemptIndex)
	}
}

func TestIdleBeyondTTLStartsNewSession(t *testing.T) {
	reg := NewRegistry(10*time.Minute, 0, 0)
	now := baseNow()
	first := reg.Resolve(Input{UserID: 5, APIKeyID: 1, FirstUserPrompt: "同一句话", Now: now})
	again := reg.Resolve(Input{UserID: 5, APIKeyID: 1, FirstUserPrompt: "同一句话", Now: now.Add(11 * time.Minute)})
	if first.SessionID == again.SessionID {
		t.Fatal("空闲超过 TTL 应视为新会话，不能让同一个锚点永久黏在一起")
	}
	if again.StepIndex != 1 {
		t.Fatalf("新会话 step = %d, want 1", again.StepIndex)
	}
}

func TestNoAnchorNoSession(t *testing.T) {
	reg := NewRegistry(0, 0, 0)
	grp := reg.Resolve(Input{UserID: 1, APIKeyID: 2, Now: baseNow()})
	if grp.SessionID != "" || grp.ThreadID != "" {
		t.Fatalf("没有会话头也没有提示词时不得凭空造会话：%+v", grp)
	}
	if grp.SessionSource != SourceNone {
		t.Fatalf("source = %q", grp.SessionSource)
	}
}

func TestUserAndAPIKeySeparateSessions(t *testing.T) {
	reg := NewRegistry(0, 0, 0)
	now := baseNow()
	a := reg.Resolve(Input{UserID: 1, APIKeyID: 1, FirstUserPrompt: "同样的开场白", Now: now})
	b := reg.Resolve(Input{UserID: 2, APIKeyID: 1, FirstUserPrompt: "同样的开场白", Now: now})
	if a.SessionID == b.SessionID {
		t.Fatal("不同用户不得因为开场白相同被并进同一个会话")
	}
}

func TestTimeRegressionDoesNotRewindStep(t *testing.T) {
	reg := NewRegistry(0, 0, 0)
	now := baseNow()
	reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, RequestFingerprint: "a", Now: now.Add(time.Minute)})
	late := reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, RequestFingerprint: "b", Now: now})
	if late.StepIndex != 2 {
		t.Fatalf("乱序批次不应把步骤算回过去：step=%d", late.StepIndex)
	}
}

func TestRegistryStaysBounded(t *testing.T) {
	reg := NewRegistry(time.Minute, 0, 8)
	now := baseNow()
	for i := 0; i < 200; i++ {
		reg.Resolve(Input{
			ClientSessionID: "s" + string(rune('a'+i%26)),
			UserID:          int64(i),
			FirstUserPrompt: "提示词" + string(rune('a'+i%26)),
			Now:             now.Add(time.Duration(i) * 2 * time.Minute),
		})
	}
	// 线程表与轮次表各自有界：两张表都不能随请求量无界增长。
	if got := len(reg.threads); got > 8 {
		t.Fatalf("threads = %d，超过上限必须淘汰", got)
	}
	if got := len(reg.turns); got > 8 {
		t.Fatalf("turns = %d，超过上限必须淘汰", got)
	}
}

// TestUserTurnsSplitsTurns 覆盖新的轮次维度：
// 用户再次开口（条数 +1）必须换成新的 TurnID，步骤从 1 重新开始。
func TestUserTurnsSplitsTurns(t *testing.T) {
	reg := NewRegistry(0, 0, 0)
	now := baseNow()
	first := reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, UserTurns: 1, RequestFingerprint: "a", Now: now})
	second := reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, UserTurns: 1, RequestFingerprint: "b", Now: now.Add(time.Minute)})
	next := reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, UserTurns: 2, RequestFingerprint: "c", Now: now.Add(2 * time.Minute)})

	if first.TurnID == "" || first.TurnID != second.TurnID {
		t.Fatalf("同一轮内的 TurnID 必须相同：%q vs %q", first.TurnID, second.TurnID)
	}
	if next.TurnID == first.TurnID {
		t.Fatal("用户再次开口必须换成新的 TurnID")
	}
	if second.StepIndex != 2 {
		t.Fatalf("同轮第二步 step = %d, want 2", second.StepIndex)
	}
	if next.StepIndex != 1 {
		t.Fatalf("新一轮 step = %d, want 1（步序必须按轮重置）", next.StepIndex)
	}
	if next.TurnIndex != 2 || first.TurnIndex != 1 {
		t.Fatalf("turnIndex = %d/%d, want 1/2", first.TurnIndex, next.TurnIndex)
	}
	// 会话聚合键跨轮稳定：否则 session 视图会被打断。
	if first.SessionID != next.SessionID {
		t.Fatalf("同一线程的会话 id 必须跨轮稳定：%q vs %q", first.SessionID, next.SessionID)
	}
}

// TestInterleavedTurnCountsDoNotInterfere 覆盖实测里最关键的坑：
// 同一线程里两个并发子代理的条数在 3 和 14 之间交替，
// 两路必须各自累计步序，谁也不能把对方的步骤吃掉。
func TestInterleavedTurnCountsDoNotInterfere(t *testing.T) {
	reg := NewRegistry(0, 0, 0)
	now := baseNow()
	var stepsA, stepsB []int
	for i := 0; i < 6; i++ {
		turns := 3
		if i%2 == 1 {
			turns = 14
		}
		grp := reg.Resolve(Input{
			ClientSessionID:    "s1",
			UserID:             1,
			FirstUserPrompt:    "并行子代理",
			UserTurns:          turns,
			RequestFingerprint: "fp-" + string(rune('a'+i)),
			Now:                now.Add(time.Duration(i) * time.Minute),
		})
		if turns == 3 {
			stepsA = append(stepsA, grp.StepIndex)
		} else {
			stepsB = append(stepsB, grp.StepIndex)
		}
	}
	want := []int{1, 2, 3}
	for i := range want {
		if stepsA[i] != want[i] {
			t.Fatalf("条数 3 的流步序 = %v, want %v", stepsA, want)
		}
		if stepsB[i] != want[i] {
			t.Fatalf("条数 14 的流步序 = %v, want %v", stepsB, want)
		}
	}
}

// TestTurnIdleResetsStepCount：一轮空闲超过阈值后再来同样的条数，
// 必须当作新一轮重新开步序，否则卡片会显示从第 7 步开始。
func TestTurnIdleResetsStepCount(t *testing.T) {
	reg := NewRegistry(time.Hour, 2*time.Minute, 0)
	now := baseNow()
	reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, UserTurns: 1, RequestFingerprint: "a", Now: now})
	reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, UserTurns: 1, RequestFingerprint: "b", Now: now.Add(30 * time.Second)})
	after := reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, UserTurns: 1, RequestFingerprint: "c", Now: now.Add(3 * time.Minute)})
	if after.StepIndex != 1 {
		t.Fatalf("空闲超过 turnIdle 后 step = %d, want 1", after.StepIndex)
	}
}

func TestTitleKeepsFirstLine(t *testing.T) {
	got := title("第一行标题\n第二行正文")
	if got != "第一行标题" {
		t.Fatalf("title = %q", got)
	}
	long := strings.Repeat("字", 500)
	if got := title(long); len(got) > titleLimit+3 {
		t.Fatalf("长标题未截断：len=%d", len(got))
	}
}

func TestTitlePrefersRequestSection(t *testing.T) {
	got := title("# Files mentioned by the user:\n- a.svg\n\n## My request:\n看下这张图对应哪组实验数据")
	if !strings.HasPrefix(got, "看下这张图") {
		t.Fatalf("title = %q，应取 ## My request 之后的内容", got)
	}
}

func TestThreadAnchorSurvivesInjectedContextRewrite(t *testing.T) {
	reg := NewRegistry(0, 0, 0)
	now := baseNow()
	// 同一段对话：上游压缩上下文后，注入部分变了，用户那句话没变。
	a := reg.Resolve(Input{ClientSessionID: "s9", UserID: 3, FirstUserPrompt: "## My request:\n把登录修好", Now: now})
	b := reg.Resolve(Input{
		ClientSessionID: "s9", UserID: 3,
		FirstUserPrompt: "# Files mentioned by the user:\n- a.go\n\n## My request:\n把登录修好",
		Now:             now.Add(time.Minute),
	})
	if a.ThreadID != b.ThreadID {
		t.Fatalf("请求段落相同必须仍是同一线程：%s vs %s", a.ThreadID, b.ThreadID)
	}
	if b.StepIndex != 2 {
		t.Fatalf("step = %d, want 2", b.StepIndex)
	}
}

func TestThreadAnchorUsesTitleLine(t *testing.T) {
	reg := NewRegistry(0, 0, 0)
	now := baseNow()
	// 同一任务：上游改写首条消息后续内容，首行不变。
	a := reg.Resolve(Input{ClientSessionID: "s10", UserID: 4, FirstUserPrompt: "# Codex 任务：PP 审计\n\n第一版上下文", Now: now})
	b := reg.Resolve(Input{
		ClientSessionID: "s10", UserID: 4,
		FirstUserPrompt: "# Codex 任务：PP 审计\n\n压缩后的上下文，内容完全不同",
		Now:             now.Add(time.Minute),
	})
	if a.ThreadID != b.ThreadID {
		t.Fatalf("首行相同必须仍是同一线程：%s vs %s", a.ThreadID, b.ThreadID)
	}
	if b.StepIndex != 2 {
		t.Fatalf("step = %d, want 2", b.StepIndex)
	}
}

func TestTitleDoesNotAddEllipsisForLaterLines(t *testing.T) {
	if got := title("第一行标题\n第二行正文"); got != "第一行标题" {
		t.Fatalf("title = %q，多行正文不该加省略号", got)
	}
}

// TestSessionIDIsClamped 覆盖客户端可控会话头的收口：
// 它同时是常驻 map key 和 trace 上的字段，1MB 的 header 能直接把上报批次顶穿。
func TestSessionIDIsClamped(t *testing.T) {
	reg := NewRegistry(0, 0, 0)
	huge := strings.Repeat("s", 4096)
	grp := reg.Resolve(Input{ClientSessionID: huge, UserID: 1, UserTurns: 1, Now: baseNow()})

	if len(grp.SessionID) > sessionIDLimit+16 {
		t.Fatalf("sessionID 长度 = %d，未按 %d 收口", len(grp.SessionID), sessionIDLimit)
	}
	// 截断会为 "#"+指纹留位，所以保留的前缀比上限略短。
	if keep := sessionIDLimit - 9; !strings.HasPrefix(grp.SessionID, huge[:keep]) {
		t.Fatalf("截断后应保留前 %d 字节前缀：%q", keep, grp.SessionID)
	}
	// 两个只在尾部不同的超长 id 不能被截成同一个。
	other := reg.Resolve(Input{ClientSessionID: huge + "x", UserID: 1, UserTurns: 1, Now: baseNow()})
	if grp.SessionID == other.SessionID {
		t.Fatal("截断后丢了区分度：不同的会话头落到同一个 id")
	}
	// 换行会污染 UI 与 tag，必须被抹掉。
	withNewline := reg.Resolve(Input{ClientSessionID: "a\nb", UserID: 1, UserTurns: 1, Now: baseNow()})
	if strings.ContainsAny(withNewline.SessionID, "\r\n\t") {
		t.Fatalf("sessionID 仍含控制字符：%q", withNewline.SessionID)
	}
}

// TestTurnIdleComesFromInput 覆盖"落卡阈值由调用方传入"：
// 观测层与归组必须用同一个阈值，不能一个读全局、一个读配置。
func TestTurnIdleComesFromInput(t *testing.T) {
	reg := NewRegistry(time.Hour, DefaultTurnIdle, 0)
	now := baseNow()
	reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, UserTurns: 1, RequestFingerprint: "a", TurnIdle: 10 * time.Second, Now: now})
	// 距上次 30s：用默认 5 分钟看是同一轮，用传入的 10s 看是新的一轮。
	next := reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, UserTurns: 1, RequestFingerprint: "b", TurnIdle: 10 * time.Second, Now: now.Add(30 * time.Second)})
	if next.StepIndex != 1 {
		t.Fatalf("step = %d, want 1（步序重置必须跟随传入的 TurnIdle）", next.StepIndex)
	}
}
