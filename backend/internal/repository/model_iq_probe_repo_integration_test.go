//go:build integration

package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/ent/modeliqproberesult"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 智商探针 repo 层集成测试：
//   - Insert（ent 写入）→ ListPage/ListRecent（裸 SQL 列）的列顺序与 scan 一致性
//   - 生效状态过滤（override_status 优先于 status）
//   - OverrideStatus 设置与清除
//   - DeleteBefore 保留期清理
//
// 裸 SQL 直连全局 client（不识别 tx ctx），不走 testEntTx 回滚，用 t.Cleanup 清理。

func TestModelIQProbeResultRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := NewModelIQProbeRepository(integrationEntClient, integrationDB)

	passStatus := service.ModelIQProbeStatusPass
	rows := []*service.ModelIQProbeResult{
		{
			AccountID:      int64Ptr(42),
			AccountName:    "openai-main",
			Model:          "gpt-5.2",
			Protocol:       "responses",
			Status:         service.ModelIQProbeStatusPass,
			ExpectedAnswer: "21",
			ActualAnswer:   "所以最少取出 21 个糖果。",
			LatencyMs:      1234,
			CreatedAt:      time.Now().Add(-2 * time.Hour),
		},
		{
			AccountID:      int64Ptr(42),
			AccountName:    "openai-main",
			Model:          "gpt-5.2",
			Protocol:       "chat_completions",
			Status:         service.ModelIQProbeStatusWrongAnswer,
			ExpectedAnswer: "21",
			ActualAnswer:   "答案是 20 个。",
			LatencyMs:      2345,
			CreatedAt:      time.Now().Add(-1 * time.Hour),
		},
		{
			// account_id 为 NULL 的行（无账号目标）。
			Model:          "gpt-5.1",
			Status:         service.ModelIQProbeStatusError,
			ErrorMessage:   "upstream 500",
			ExpectedAnswer: "21",
			CreatedAt:      time.Now().Add(-30 * time.Minute),
		},
	}
	for i := range rows {
		require.NoError(t, repo.Insert(ctx, rows[i]))
		require.NotZero(t, rows[i].ID)
	}
	t.Cleanup(func() {
		_, _ = integrationEntClient.ModelIQProbeResult.Delete().Exec(ctx)
	})

	// ListRecent：按 created_at desc，全列回读。
	// ListRecent：按 created_at desc，全列回读。
	// recent[0]=gpt-5.1（30min 前），recent[1]=wrong_answer 行（1h 前），recent[2]=pass 行（2h 前）。
	recent, err := repo.ListRecent(ctx, 10)
	require.NoError(t, err)
	require.Len(t, recent, 3)
	require.Equal(t, "gpt-5.1", recent[0].Model)
	require.Nil(t, recent[0].AccountID)
	require.Equal(t, "答案是 20 个。", recent[1].ActualAnswer)
	require.NotNil(t, recent[1].AccountID)
	require.Equal(t, int64(42), *recent[1].AccountID)
	require.Equal(t, "openai-main", recent[1].AccountName)
	require.Equal(t, int64(2345), recent[1].LatencyMs)
	require.Equal(t, "所以最少取出 21 个糖果。", recent[2].ActualAnswer)
	require.Equal(t, int64(1234), recent[2].LatencyMs)
	require.Equal(t, time.Duration(0), recent[2].CreatedAt.Sub(rows[0].CreatedAt))

	// ListPage：按模型过滤 + 分页。
	page1, total, err := repo.ListPage(ctx, 1, 1, service.ModelIQProbeListFilters{Model: "gpt-5.2"})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, page1, 1)

	// ListPage：生效状态过滤（未改判时 = status）。
	wrongRows, wrongTotal, err := repo.ListPage(ctx, 1, 10, service.ModelIQProbeListFilters{Status: service.ModelIQProbeStatusWrongAnswer})
	require.NoError(t, err)
	require.EqualValues(t, 1, wrongTotal)
	require.Equal(t, "答案是 20 个。", wrongRows[0].ActualAnswer)

	// 人工改判：wrong_answer 行改判为 pass 后，pass 过滤应命中它。
	require.NoError(t, repo.OverrideStatus(ctx, rows[1].ID, &passStatus, nil))
	passRows, passTotal, err := repo.ListPage(ctx, 1, 10, service.ModelIQProbeListFilters{Status: service.ModelIQProbeStatusPass})
	require.NoError(t, err)
	require.EqualValues(t, 2, passTotal)
	// created_at desc：改判行（1h 前）在原 pass 行（2h 前）之前。
	require.Equal(t, "答案是 20 个。", passRows[0].ActualAnswer)
	require.Equal(t, "所以最少取出 21 个糖果。", passRows[1].ActualAnswer)

	// 清除改判恢复自动判定。
	require.NoError(t, repo.OverrideStatus(ctx, rows[1].ID, nil, nil))
	_, passTotal, err = repo.ListPage(ctx, 1, 10, service.ModelIQProbeListFilters{Status: service.ModelIQProbeStatusPass})
	require.NoError(t, err)
	require.EqualValues(t, 1, passTotal)

	// 改判同时改写回答原文；GetByID 可读回。
	rewritten := "所以最少取出 21 个糖果（星形 12 + 圆形 9）。"
	require.NoError(t, repo.OverrideStatus(ctx, rows[1].ID, &passStatus, &rewritten))
	rewrittenRow, err := repo.GetByID(ctx, rows[1].ID)
	require.NoError(t, err)
	require.Equal(t, rewritten, rewrittenRow.ActualAnswer)
	require.Equal(t, "pass", rewrittenRow.ResolveEffectiveStatus())

	// RandomPassAnswer：优先同模型，排除自身，只取有效通过行。
	sample, ok, err := repo.RandomPassAnswer(ctx, "gpt-5.2", rows[1].ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "所以最少取出 21 个糖果。", sample)

	// 改判不存在的行 → NotFound。
	require.ErrorIs(t, repo.OverrideStatus(ctx, 999999, &passStatus, nil), service.ErrModelIQProbeResultNotFound)

	// DeleteBefore：只删截止时间前的行。
	deleted, err := repo.DeleteBefore(ctx, time.Now().Add(-45*time.Minute))
	require.NoError(t, err)
	require.EqualValues(t, 2, deleted)
	remaining, _, err := repo.ListPage(ctx, 1, 10, service.ModelIQProbeListFilters{})
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	require.Equal(t, "gpt-5.1", remaining[0].Model)
}

func int64Ptr(v int64) *int64 {
	return &v
}

func TestModelIQProbeSummaryPreservesOlderTargetFailure(t *testing.T) {
	ctx := context.Background()
	repo := NewModelIQProbeRepository(integrationEntClient, integrationDB)
	const model = "gpt-iq-summary-test"
	t.Cleanup(func() {
		_, err := integrationEntClient.ModelIQProbeResult.Delete().Where(modeliqproberesult.ModelEQ(model)).Exec(ctx)
		require.NoError(t, err)
	})
	now := time.Now()
	for _, row := range []*service.ModelIQProbeResult{
		{Model: model, AccountID: int64Ptr(101), Status: service.ModelIQProbeStatusWrongAnswer, ExpectedAnswer: "21", CreatedAt: now.Add(-time.Hour)},
		{Model: model, AccountID: int64Ptr(102), Status: service.ModelIQProbeStatusPass, ExpectedAnswer: "21", CreatedAt: now.Add(-2 * time.Minute)},
		{Model: model, AccountID: int64Ptr(102), Status: service.ModelIQProbeStatusPass, ExpectedAnswer: "21", CreatedAt: now.Add(-time.Minute)},
		{Model: model, AccountID: int64Ptr(102), Status: service.ModelIQProbeStatusPass, ExpectedAnswer: "21", CreatedAt: now},
		{Model: model, AccountID: int64Ptr(103), Status: service.ModelIQProbeStatusWrongAnswer, ExpectedAnswer: "21", CreatedAt: now.AddDate(0, 0, -31)},
	} {
		require.NoError(t, repo.Insert(ctx, row))
	}
	rows, err := repo.ListForSummary(ctx, []service.ModelIQProbeTargetRef{{AccountID: 101, Model: model}, {AccountID: 102, Model: model}, {AccountID: 103, Model: model}}, 2)
	require.NoError(t, err)
	// 两条模型时间线 + 另一账号的最新失败；不加载全部账号历史。
	require.Len(t, rows, 3)
	require.Equal(t, service.ModelIQProbeStatusWrongAnswer, rows[2].Status)
	rows, err = repo.ListForSummary(ctx, []service.ModelIQProbeTargetRef{{AccountID: 101, Model: model}}, 2)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	rows, err = repo.ListForSummary(ctx, []service.ModelIQProbeTargetRef{{AccountID: 101, Model: model + "-missing"}}, 2)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestModelIQProbeSummaryFiltersExactTargetPairs(t *testing.T) {
	ctx := context.Background()
	repo := NewModelIQProbeRepository(integrationEntClient, integrationDB)
	const firstModel, secondModel = "gpt-iq-pair-a", "gpt-iq-pair-b"
	t.Cleanup(func() {
		_, err := integrationEntClient.ModelIQProbeResult.Delete().Where(modeliqproberesult.ModelIn(firstModel, secondModel)).Exec(ctx)
		require.NoError(t, err)
	})
	now := time.Now()
	for _, row := range []*service.ModelIQProbeResult{
		{AccountID: int64Ptr(101), Model: firstModel, Status: service.ModelIQProbeStatusPass, ExpectedAnswer: "21", CreatedAt: now.Add(-time.Minute)},
		{AccountID: int64Ptr(102), Model: secondModel, Status: service.ModelIQProbeStatusPass, ExpectedAnswer: "21", CreatedAt: now.Add(-time.Minute)},
		{AccountID: int64Ptr(101), Model: secondModel, Status: service.ModelIQProbeStatusWrongAnswer, ExpectedAnswer: "21", CreatedAt: now},
		{AccountID: int64Ptr(102), Model: firstModel, Status: service.ModelIQProbeStatusWrongAnswer, ExpectedAnswer: "21", CreatedAt: now},
	} {
		require.NoError(t, repo.Insert(ctx, row))
	}
	rows, err := repo.ListForSummary(ctx, []service.ModelIQProbeTargetRef{{AccountID: 101, Model: firstModel}, {AccountID: 102, Model: secondModel}}, 60)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, service.ModelIQProbeStatusPass, row.Status)
	}
	models, err := repo.ListSummaryModels(ctx, []int64{101})
	require.NoError(t, err)
	require.Equal(t, []string{firstModel, secondModel}, models)
}
