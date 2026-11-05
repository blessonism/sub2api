//go:build unit

package sessiongroup

import (
	"strings"
	"testing"
	"time"
)

func baseNow() time.Time { return time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC) }

func TestClientSessionIDWins(t *testing.T) {
	reg := NewRegistry(0, 0)
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
	reg := NewRegistry(0, 0)
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
	reg := NewRegistry(0, 0)
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
	reg := NewRegistry(0, 0)
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
	reg := NewRegistry(0, 0)
	now := baseNow()
	reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, FirstUserPrompt: "x", RequestFingerprint: "same", Now: now})
	later := reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, FirstUserPrompt: "x", RequestFingerprint: "same", Now: now.Add(attemptWindow + time.Minute)})
	if later.StepIndex != 2 || later.AttemptIndex != 1 {
		t.Fatalf("超出尝试窗口的相同请求体应算新一步：step=%d attempt=%d", later.StepIndex, later.AttemptIndex)
	}
}

func TestIdleBeyondTTLStartsNewSession(t *testing.T) {
	reg := NewRegistry(10*time.Minute, 0)
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
	reg := NewRegistry(0, 0)
	grp := reg.Resolve(Input{UserID: 1, APIKeyID: 2, Now: baseNow()})
	if grp.SessionID != "" || grp.ThreadID != "" {
		t.Fatalf("没有会话头也没有提示词时不得凭空造会话：%+v", grp)
	}
	if grp.SessionSource != SourceNone {
		t.Fatalf("source = %q", grp.SessionSource)
	}
}

func TestUserAndAPIKeySeparateSessions(t *testing.T) {
	reg := NewRegistry(0, 0)
	now := baseNow()
	a := reg.Resolve(Input{UserID: 1, APIKeyID: 1, FirstUserPrompt: "同样的开场白", Now: now})
	b := reg.Resolve(Input{UserID: 2, APIKeyID: 1, FirstUserPrompt: "同样的开场白", Now: now})
	if a.SessionID == b.SessionID {
		t.Fatal("不同用户不得因为开场白相同被并进同一个会话")
	}
}

func TestTimeRegressionDoesNotRewindStep(t *testing.T) {
	reg := NewRegistry(0, 0)
	now := baseNow()
	reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, RequestFingerprint: "a", Now: now.Add(time.Minute)})
	late := reg.Resolve(Input{ClientSessionID: "s1", UserID: 1, RequestFingerprint: "b", Now: now})
	if late.StepIndex != 2 {
		t.Fatalf("乱序批次不应把步骤算回过去：step=%d", late.StepIndex)
	}
}

func TestRegistryStaysBounded(t *testing.T) {
	reg := NewRegistry(time.Minute, 8)
	now := baseNow()
	for i := 0; i < 200; i++ {
		reg.Resolve(Input{
			ClientSessionID: "s" + string(rune('a'+i%26)),
			UserID:          int64(i),
			FirstUserPrompt: "提示词" + string(rune('a'+i%26)),
			Now:             now.Add(time.Duration(i) * 2 * time.Minute),
		})
	}
	if reg.Len() > 8 {
		t.Fatalf("entries = %d，超过上限必须淘汰", reg.Len())
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
