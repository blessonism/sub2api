package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

func TestLotteryEntryCountModes(t *testing.T) {
	campaign := LotteryCampaign{
		ThresholdTokens:   100,
		EntryMode:         LotteryEntryDailyOnce,
		MaxEntriesPerUser: 1,
	}
	if got := lotteryEntryCount(campaign, 99); got != 0 {
		t.Fatalf("below threshold entries = %d, want 0", got)
	}
	if got := lotteryEntryCount(campaign, 500); got != 1 {
		t.Fatalf("daily once entries = %d, want 1", got)
	}

	campaign.EntryMode = LotteryEntryStepped
	campaign.EntryStepTokens = 50
	campaign.MaxEntriesPerUser = 4
	if got := lotteryEntryCount(campaign, 100); got != 1 {
		t.Fatalf("threshold entries = %d, want 1", got)
	}
	if got := lotteryEntryCount(campaign, 210); got != 3 {
		t.Fatalf("stepped entries = %d, want 3", got)
	}
	if got := lotteryEntryCount(campaign, 1000); got != 4 {
		t.Fatalf("capped entries = %d, want 4", got)
	}
}

func TestSelectLotteryWinnersOneWinPerBatch(t *testing.T) {
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.Local)
	campaign := LotteryCampaign{
		ID: 7,
		PrizeTiers: []LotteryPrizeTier{
			{ID: 1, CampaignID: 7, TierName: "一等奖", WinnerCount: 2, RewardAmountCents: 1000, SortOrder: 1},
			{ID: 2, CampaignID: 7, TierName: "二等奖", WinnerCount: 2, RewardAmountCents: 500, SortOrder: 2},
		},
	}
	batch := LotteryDrawBatch{ID: 9, CampaignID: 7, DrawDate: drawDate}
	candidates := []LotteryDrawCandidate{
		{UserID: 1, EntryDate: drawDate, EntryCount: 5},
		{UserID: 2, EntryDate: drawDate, EntryCount: 3},
		{UserID: 3, EntryDate: drawDate, EntryCount: 1},
	}

	winners, err := selectLotteryWinnersWithDesignations(campaign, batch, candidates, nil)
	if err != nil {
		t.Fatalf("select winners: %v", err)
	}
	if len(winners) != len(candidates) {
		t.Fatalf("winners = %d, want capped by candidates %d", len(winners), len(candidates))
	}
	seen := map[int64]bool{}
	for _, winner := range winners {
		if seen[winner.UserID] {
			t.Fatalf("user %d won more than once in the same batch", winner.UserID)
		}
		seen[winner.UserID] = true
		if winner.IdempotencyKey == "" {
			t.Fatalf("winner idempotency key is empty")
		}
	}
}

func TestLotteryBatchCanResume(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	if lotteryBatchCanResume(LotteryDrawBatch{Status: lotteryDrawStatusSuccess, CreatedAt: now.Add(-time.Hour)}, now) {
		t.Fatalf("success batch should not resume")
	}
	if lotteryBatchCanResume(LotteryDrawBatch{Status: lotteryDrawStatusRunning, CreatedAt: now.Add(-time.Minute)}, now) {
		t.Fatalf("fresh processing batch should not resume")
	}
	if !lotteryBatchCanResume(LotteryDrawBatch{Status: lotteryDrawStatusRunning, CreatedAt: now.Add(-time.Hour)}, now) {
		t.Fatalf("stale processing batch should resume")
	}
	if !lotteryBatchCanResume(LotteryDrawBatch{Status: lotteryDrawStatusFailed, CreatedAt: now.Add(-time.Minute)}, now) {
		t.Fatalf("failed batch should resume")
	}
}

func TestLotteryCreateRejectsSingleDrawOutsideWindow(t *testing.T) {
	start := time.Date(2026, 7, 8, 9, 0, 0, 0, time.UTC)
	input := lotteryValidInput(start)
	drawAt := start.Add(-time.Minute)
	input.DrawAt = &drawAt
	svc := NewLotteryCampaignService(&lotteryServiceRepoStub{}, nil)
	if _, err := svc.Create(context.Background(), input); !errors.Is(err, ErrLotteryInvalidConfig) {
		t.Fatalf("create err = %v, want ErrLotteryInvalidConfig", err)
	}
}

func TestLotteryCreateAllowsOneDayDailyCampaignWhenDrawInWindow(t *testing.T) {
	start := time.Date(2026, 7, 8, 9, 0, 0, 0, time.UTC)
	input := lotteryValidInput(start)
	input.DrawScheduleType = LotteryDrawDaily
	input.DrawAt = nil
	input.DailyDrawTime = "20:00"
	input.EndAt = time.Date(2026, 7, 8, 23, 0, 0, 0, time.UTC)
	repo := &lotteryServiceRepoStub{}
	svc := NewLotteryCampaignService(repo, nil)
	if _, err := svc.Create(context.Background(), input); err != nil {
		t.Fatalf("create one-day daily campaign failed: %v", err)
	}
}

func TestLotteryCreateRejectsDailyCampaignWithoutDrawInWindow(t *testing.T) {
	start := time.Date(2026, 7, 8, 9, 0, 0, 0, time.UTC)
	input := lotteryValidInput(start)
	input.DrawScheduleType = LotteryDrawDaily
	input.DrawAt = nil
	input.DailyDrawTime = "08:00"
	input.EndAt = time.Date(2026, 7, 8, 23, 0, 0, 0, time.UTC)
	svc := NewLotteryCampaignService(&lotteryServiceRepoStub{}, nil)
	if _, err := svc.Create(context.Background(), input); !errors.Is(err, ErrLotteryInvalidConfig) {
		t.Fatalf("create err = %v, want ErrLotteryInvalidConfig", err)
	}
}

func TestLotteryFeatureRequiresPublishedActiveCampaign(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{ID: 7, Status: LotteryStatusPublished, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour)},
	}
	svc := NewLotteryCampaignService(repo, nil)
	featured, err := svc.Feature(context.Background(), 7, nil, now)
	if err != nil {
		t.Fatalf("feature active campaign failed: %v", err)
	}
	if featured == nil || repo.featureCalls != 1 {
		t.Fatalf("feature calls = %d, campaign = %+v", repo.featureCalls, featured)
	}

	repo.campaign.StartAt = now.Add(time.Hour)
	if _, err := svc.Feature(context.Background(), 7, nil, now); !errors.Is(err, ErrLotteryFeaturedInvalid) {
		t.Fatalf("feature future err = %v, want ErrLotteryFeaturedInvalid", err)
	}
}

func TestLotteryDrawRejectsUnpublishedCampaign(t *testing.T) {
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{ID: 7, Status: LotteryStatusDraft},
	}
	svc := NewLotteryCampaignService(repo, nil)
	_, err := svc.Draw(context.Background(), 7, time.Now(), "manual", nil, time.Now())
	if !errors.Is(err, ErrLotteryNotPublished) {
		t.Fatalf("draw err = %v, want ErrLotteryNotPublished", err)
	}
	if repo.createBatchCalls != 0 || repo.grantCalls != 0 {
		t.Fatalf("unpublished draw should not create batch or grant, got batches=%d grants=%d", repo.createBatchCalls, repo.grantCalls)
	}
}

func TestLotteryDrawRejectsBeforeScheduledTime(t *testing.T) {
	now := time.Date(2026, 7, 8, 19, 0, 0, 0, time.UTC)
	drawAt := now.Add(time.Hour)
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{ID: 7, Status: LotteryStatusPublished, DrawScheduleType: LotteryDrawSingle, DrawAt: lotteryPtrTime(drawAt)},
	}
	svc := NewLotteryCampaignService(repo, nil)
	_, err := svc.Draw(context.Background(), 7, drawDate, "manual", nil, now)
	if !errors.Is(err, ErrLotteryDrawNotDue) {
		t.Fatalf("draw err = %v, want ErrLotteryDrawNotDue", err)
	}
	if repo.createBatchCalls != 0 || repo.createWinnersCalls != 0 || repo.grantCalls != 0 {
		t.Fatalf("early draw should not mutate state, got batches=%d winners=%d grants=%d", repo.createBatchCalls, repo.createWinnersCalls, repo.grantCalls)
	}
}

func TestLotteryDrawResumesFailedBatchWithoutDuplicateSuccessGrant(t *testing.T) {
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{ID: 7, Name: "Token 抽奖", Status: LotteryStatusPublished, DrawScheduleType: LotteryDrawSingle, DrawAt: lotteryPtrTime(drawDate.Add(20 * time.Hour))},
		batch: &LotteryDrawBatch{
			ID:         9,
			CampaignID: 7,
			DrawDate:   drawDate,
			BatchNo:    "lottery-7-20260708",
			Status:     lotteryDrawStatusPartial,
			CreatedAt:  drawDate.Add(20 * time.Hour),
		},
		winners: []LotteryWinner{
			{ID: 1, BatchID: 9, CampaignID: 7, UserID: 11, PrizeName: "一等奖", RewardAmountCents: 1000, Status: lotteryWinnerStatusSuccess},
			{ID: 2, BatchID: 9, CampaignID: 7, UserID: 12, PrizeName: "二等奖", RewardAmountCents: 500, Status: "failed"},
		},
	}
	svc := NewLotteryCampaignService(repo, nil)
	batch, err := svc.Draw(context.Background(), 7, drawDate, "manual", nil, drawDate.Add(21*time.Hour))
	if err != nil {
		t.Fatalf("draw failed: %v", err)
	}
	if repo.createWinnersCalls != 0 {
		t.Fatalf("resuming failed batch should not reselect winners, got %d calls", repo.createWinnersCalls)
	}
	if repo.grantCalls != 1 {
		t.Fatalf("grant calls = %d, want retry only failed winner once", repo.grantCalls)
	}
	if batch.Status != lotteryDrawStatusSuccess {
		t.Fatalf("batch status = %s, want success", batch.Status)
	}
}

func TestLotteryDrawFreshProcessingBatchReturnsWithoutGrant(t *testing.T) {
	now := time.Date(2026, 7, 8, 21, 0, 0, 0, time.UTC)
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{ID: 7, Status: LotteryStatusPublished, DrawScheduleType: LotteryDrawSingle, DrawAt: lotteryPtrTime(now.Add(-time.Hour))},
		batch: &LotteryDrawBatch{
			ID:         9,
			CampaignID: 7,
			DrawDate:   drawDate,
			BatchNo:    "lottery-7-20260708",
			Status:     lotteryDrawStatusRunning,
			CreatedAt:  now.Add(-time.Minute),
		},
		winners: []LotteryWinner{{ID: 2, BatchID: 9, CampaignID: 7, UserID: 12, PrizeName: "二等奖", RewardAmountCents: 500, Status: "pending"}},
	}
	svc := NewLotteryCampaignService(repo, nil)
	batch, err := svc.Draw(context.Background(), 7, drawDate, "manual", nil, now)
	if err != nil {
		t.Fatalf("draw failed: %v", err)
	}
	if batch.Status != lotteryDrawStatusRunning {
		t.Fatalf("batch status = %s, want processing", batch.Status)
	}
	if repo.grantCalls != 0 {
		t.Fatalf("fresh processing batch should not grant, got %d calls", repo.grantCalls)
	}
}

func TestLotteryDrawResumesStaleProcessingBatchWithExistingWinners(t *testing.T) {
	now := time.Date(2026, 7, 8, 21, 0, 0, 0, time.UTC)
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{
			ID:               7,
			Name:             "Token 抽奖",
			Status:           LotteryStatusPublished,
			DrawScheduleType: LotteryDrawSingle,
			DrawAt:           lotteryPtrTime(now.Add(-time.Hour)),
			PrizeTiers:       []LotteryPrizeTier{{ID: 1, TierName: "一等奖", WinnerCount: 1, RewardAmountCents: 1000, SortOrder: 1}},
		},
		batch: &LotteryDrawBatch{
			ID:         9,
			CampaignID: 7,
			DrawDate:   drawDate,
			BatchNo:    "lottery-7-20260708",
			Status:     lotteryDrawStatusRunning,
			CreatedAt:  now.Add(-time.Hour),
		},
		winners: []LotteryWinner{{ID: 2, BatchID: 9, CampaignID: 7, UserID: 12, PrizeName: "一等奖", RewardAmountCents: 1000, Status: "pending"}},
		candidates: []LotteryDrawCandidate{
			{UserID: 12, EntryDate: drawDate, EntryCount: 1},
			{UserID: 13, EntryDate: drawDate, EntryCount: 1},
		},
	}
	svc := NewLotteryCampaignService(repo, nil)
	batch, err := svc.Draw(context.Background(), 7, drawDate, "manual", nil, now)
	if err != nil {
		t.Fatalf("draw failed: %v", err)
	}
	if repo.createWinnersCalls != 0 {
		t.Fatalf("stale processing batch with existing winners should not reselect winners, got %d calls", repo.createWinnersCalls)
	}
	if repo.grantCalls != 1 {
		t.Fatalf("grant calls = %d, want existing pending winner granted once", repo.grantCalls)
	}
	if len(repo.winners) != 1 {
		t.Fatalf("winners = %d, want no extra winners created", len(repo.winners))
	}
	if batch.Status != lotteryDrawStatusSuccess {
		t.Fatalf("batch status = %s, want success", batch.Status)
	}
}

func TestLotteryDrawInvalidatesBalanceCachesAfterSuccessfulGrant(t *testing.T) {
	now := time.Date(2026, 7, 8, 21, 0, 0, 0, time.UTC)
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{
			ID:               7,
			Name:             "Token 抽奖",
			Status:           LotteryStatusPublished,
			DrawScheduleType: LotteryDrawSingle,
			DrawAt:           lotteryPtrTime(now.Add(-time.Hour)),
			PrizeTiers:       []LotteryPrizeTier{{ID: 1, TierName: "一等奖", WinnerCount: 1, RewardAmountCents: 1000, SortOrder: 1}},
		},
		candidates: []LotteryDrawCandidate{{UserID: 12, EntryDate: drawDate, EntryCount: 1}},
	}
	cache := &lotteryBalanceGrantCacheStub{}
	svc := NewLotteryCampaignService(repo, cache)
	_, err := svc.Draw(context.Background(), 7, drawDate, "manual", nil, now)
	if err != nil {
		t.Fatalf("draw failed: %v", err)
	}
	if len(cache.invalidatedUserIDs) != 1 || cache.invalidatedUserIDs[0] != 12 {
		t.Fatalf("invalidated users = %v, want [12]", cache.invalidatedUserIDs)
	}
}

// TestLotteryDrawHonorsDesignationsThenFillsRandomly 覆盖发钱主链路：
// Draw() 读取预置名单 → 指定者优先中奖 → 剩余名额随机补齐 → 全部发放。
func TestLotteryDrawHonorsDesignationsThenFillsRandomly(t *testing.T) {
	now := time.Date(2026, 7, 8, 21, 0, 0, 0, time.UTC)
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{
			ID:               7,
			Name:             "Token 抽奖",
			Status:           LotteryStatusPublished,
			DrawScheduleType: LotteryDrawSingle,
			DrawAt:           lotteryPtrTime(now.Add(-time.Hour)),
			PrizeTiers:       []LotteryPrizeTier{{ID: 1, TierName: "一等奖", WinnerCount: 2, RewardAmountCents: 1000, SortOrder: 1}},
		},
		candidates: []LotteryDrawCandidate{
			{UserID: 11, EntryDate: drawDate, EntryCount: 1},
			{UserID: 12, EntryDate: drawDate, EntryCount: 1},
			{UserID: 13, EntryDate: drawDate, EntryCount: 1},
		},
		designations: []LotteryWinnerDesignation{{CampaignID: 7, DrawDate: drawDate, UserID: 11, PrizeTierID: 1}},
	}
	cache := &lotteryBalanceGrantCacheStub{}
	svc := NewLotteryCampaignService(repo, cache)
	batch, err := svc.Draw(context.Background(), 7, drawDate, "manual", nil, now)
	if err != nil {
		t.Fatalf("draw failed: %v", err)
	}
	if repo.createWinnersCalls != 1 {
		t.Fatalf("createWinners calls = %d, want 1", repo.createWinnersCalls)
	}
	if len(repo.winners) != 2 {
		t.Fatalf("winners = %d, want 2 (1 designated + 1 random for a 2-slot tier)", len(repo.winners))
	}
	designatedWon := false
	for _, w := range repo.winners {
		if w.UserID == 11 {
			designatedWon = true
		}
		if w.Status != lotteryWinnerStatusSuccess {
			t.Fatalf("winner %d status = %s, want success (all winners must be granted)", w.UserID, w.Status)
		}
	}
	if !designatedWon {
		t.Fatalf("designated user 11 must be among winners, got %+v", repo.winners)
	}
	if repo.grantCalls != 2 {
		t.Fatalf("grant calls = %d, want 2", repo.grantCalls)
	}
	if batch.Status != lotteryDrawStatusSuccess {
		t.Fatalf("batch status = %s, want success", batch.Status)
	}
}

func TestLotteryEnrollRejectsClosedSingleDrawWindow(t *testing.T) {
	now := time.Date(2026, 7, 8, 21, 0, 0, 0, time.UTC)
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{
			ID:                7,
			Status:            LotteryStatusPublished,
			ParticipationMode: LotteryParticipationManual,
			DrawScheduleType:  LotteryDrawSingle,
			EntryMode:         LotteryEntryDailyOnce,
			ThresholdTokens:   100,
			StartAt:           now.Add(-24 * time.Hour),
			EndAt:             now.Add(24 * time.Hour),
			DrawAt:            lotteryPtrTime(now.Add(-time.Hour)),
		},
		tokens: 200,
	}
	svc := NewLotteryCampaignService(repo, nil)
	_, err := svc.Enroll(context.Background(), 7, 42, now)
	if !errors.Is(err, ErrLotteryEntriesClosed) {
		t.Fatalf("enroll err = %v, want ErrLotteryEntriesClosed", err)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("closed enroll should not upsert entries, got %d", repo.upsertCalls)
	}
}

func TestLotteryMyDataSingleDrawUsesCampaignWindow(t *testing.T) {
	start := time.Date(2026, 7, 7, 10, 30, 0, 0, timezone.Location())
	drawAt := time.Date(2026, 7, 8, 20, 0, 0, 0, timezone.Location())
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, timezone.Location())
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{
			ID:                7,
			Status:            LotteryStatusPublished,
			ParticipationMode: LotteryParticipationAuto,
			DrawScheduleType:  LotteryDrawSingle,
			EntryMode:         LotteryEntryDailyOnce,
			ThresholdTokens:   100,
			StartAt:           start,
			EndAt:             drawAt.Add(time.Hour),
			DrawAt:            lotteryPtrTime(drawAt),
		},
		tokens:           200,
		participantCount: 12,
	}
	svc := NewLotteryCampaignService(repo, nil)

	data, err := svc.MyData(context.Background(), 7, 42, now)
	if err != nil {
		t.Fatalf("my data failed: %v", err)
	}
	if !repo.userTokensStartAt.Equal(start) || !repo.userTokensEndAt.Equal(now) {
		t.Fatalf("token window = %s - %s, want %s - %s", repo.userTokensStartAt, repo.userTokensEndAt, start, now)
	}
	if repo.upsertCalls != 1 || !repo.upsertEntryDate.Equal(dateOnly(drawAt)) {
		t.Fatalf("upsert calls/date = %d/%s, want 1/%s", repo.upsertCalls, repo.upsertEntryDate, dateOnly(drawAt))
	}
	if data.TodayTokens != 200 || data.EntryCount != 1 {
		t.Fatalf("my data tokens/entries = %d/%d, want 200/1", data.TodayTokens, data.EntryCount)
	}
	if data.ParticipantCount != 12 || !repo.participantStartArg.Equal(start) || !repo.participantEndArg.Equal(now) {
		t.Fatalf("participant count/window = %d/%s-%s, want 12/%s-%s", data.ParticipantCount, repo.participantStartArg, repo.participantEndArg, start, now)
	}
}

func TestLotterySyncEntriesSingleDrawUsesCampaignWindow(t *testing.T) {
	start := time.Date(2026, 7, 7, 10, 30, 0, 0, timezone.Location())
	drawAt := time.Date(2026, 7, 8, 20, 0, 0, 0, timezone.Location())
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{
			ID:                7,
			Status:            LotteryStatusPublished,
			ParticipationMode: LotteryParticipationAuto,
			DrawScheduleType:  LotteryDrawSingle,
			EntryMode:         LotteryEntryStepped,
			ThresholdTokens:   100,
			EntryStepTokens:   50,
			MaxEntriesPerUser: 3,
			StartAt:           start,
			EndAt:             drawAt.Add(time.Hour),
			DrawAt:            lotteryPtrTime(drawAt),
		},
		qualified: []LotteryQualifiedUsage{{UserID: 42, Tokens: 210}},
	}
	svc := NewLotteryCampaignService(repo, nil)

	count, err := svc.SyncEntries(context.Background(), 7, drawAt.AddDate(0, 0, -1), drawAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("sync entries failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("sync count = %d, want 1", count)
	}
	if !repo.qualifiedStartAt.Equal(start) || !repo.qualifiedEndAt.Equal(drawAt) {
		t.Fatalf("qualified window = %s - %s, want %s - %s", repo.qualifiedStartAt, repo.qualifiedEndAt, start, drawAt)
	}
	if !repo.upsertEntryDate.Equal(dateOnly(drawAt)) {
		t.Fatalf("entry date = %s, want %s", repo.upsertEntryDate, dateOnly(drawAt))
	}
}

func TestLotterySyncEntriesDailyDrawKeepsDrawDayWindow(t *testing.T) {
	start := time.Date(2026, 7, 7, 10, 30, 0, 0, timezone.Location())
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, timezone.Location())
	scheduledAt := time.Date(2026, 7, 8, 20, 0, 0, 0, timezone.Location())
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{
			ID:                7,
			Status:            LotteryStatusPublished,
			ParticipationMode: LotteryParticipationAuto,
			DrawScheduleType:  LotteryDrawDaily,
			DailyDrawTime:     "20:00",
			EntryMode:         LotteryEntryDailyOnce,
			ThresholdTokens:   100,
			StartAt:           start,
			EndAt:             scheduledAt.Add(24 * time.Hour),
		},
		qualified: []LotteryQualifiedUsage{{UserID: 42, Tokens: 200}},
	}
	svc := NewLotteryCampaignService(repo, nil)

	count, err := svc.SyncEntries(context.Background(), 7, drawDate, scheduledAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("sync entries failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("sync count = %d, want 1", count)
	}
	if !repo.qualifiedStartAt.Equal(timezone.StartOfDay(scheduledAt)) || !repo.qualifiedEndAt.Equal(scheduledAt) {
		t.Fatalf("qualified window = %s - %s, want %s - %s", repo.qualifiedStartAt, repo.qualifiedEndAt, timezone.StartOfDay(scheduledAt), scheduledAt)
	}
	if !repo.upsertEntryDate.Equal(drawDate) {
		t.Fatalf("entry date = %s, want %s", repo.upsertEntryDate, drawDate)
	}
}

func TestLotteryMyDataDoesNotUpsertClosedWindow(t *testing.T) {
	now := time.Date(2026, 7, 8, 21, 0, 0, 0, time.UTC)
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{
			ID:                7,
			Status:            LotteryStatusPublished,
			ParticipationMode: LotteryParticipationAuto,
			DrawScheduleType:  LotteryDrawSingle,
			EntryMode:         LotteryEntryDailyOnce,
			ThresholdTokens:   100,
			StartAt:           now.Add(-24 * time.Hour),
			EndAt:             now.Add(24 * time.Hour),
			DrawAt:            lotteryPtrTime(now.Add(-time.Hour)),
		},
		tokens: 200,
	}
	svc := NewLotteryCampaignService(repo, nil)
	data, err := svc.MyData(context.Background(), 7, 42, now)
	if err != nil {
		t.Fatalf("my data failed: %v", err)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("closed my data should not upsert entries, got %d", repo.upsertCalls)
	}
	if data.EntryCount != 0 || data.EntryStatus != "not_eligible" {
		t.Fatalf("entry state = count %d status %s, want closed window to be non-enterable", data.EntryCount, data.EntryStatus)
	}
}

func TestLotteryRecentWinnersRejectsInvisibleCampaign(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	repo := &lotteryServiceRepoStub{
		campaign: LotteryCampaign{ID: 7, Status: LotteryStatusDraft, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour)},
	}
	svc := NewLotteryCampaignService(repo, nil)
	if _, err := svc.RecentWinners(context.Background(), 7, 10, now); !errors.Is(err, ErrLotteryCampaignNotFound) {
		t.Fatalf("recent winners err = %v, want ErrLotteryCampaignNotFound", err)
	}
}

func TestLotteryRecentWinnersDefaultsAndCapsLimit(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	repo := &lotteryServiceRepoStub{
		campaign:      LotteryCampaign{ID: 7, Status: LotteryStatusPublished, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour)},
		publicWinners: []LotteryPublicWinner{{MaskedEmail: "j***@example.com", PrizeName: "Gold", RewardAmountCents: 10000, CreatedAt: now}},
	}
	svc := NewLotteryCampaignService(repo, nil)

	winners, err := svc.RecentWinners(context.Background(), 7, 0, now)
	if err != nil {
		t.Fatalf("recent winners failed: %v", err)
	}
	if repo.recentLimitArg != lotteryPublicWinnersDefaultLimit {
		t.Fatalf("limit arg = %d, want default %d", repo.recentLimitArg, lotteryPublicWinnersDefaultLimit)
	}
	if len(winners) != 1 || winners[0].MaskedEmail != "j***@example.com" || winners[0].RewardAmountCents != 10000 {
		t.Fatalf("masked winner passthrough = %+v", winners)
	}

	if _, err := svc.RecentWinners(context.Background(), 7, 999, now); err != nil {
		t.Fatalf("recent winners high limit failed: %v", err)
	}
	if repo.recentLimitArg != lotteryPublicWinnersMaxLimit {
		t.Fatalf("limit arg = %d, want capped %d", repo.recentLimitArg, lotteryPublicWinnersMaxLimit)
	}
}

type lotteryServiceRepoStub struct {
	campaign           LotteryCampaign
	batch              *LotteryDrawBatch
	winners            []LotteryWinner
	qualified          []LotteryQualifiedUsage
	candidates         []LotteryDrawCandidate
	tokens             int64
	createBatchCalls   int
	createWinnersCalls int
	grantCalls         int
	upsertCalls        int
	featureCalls       int
	deleteCalls        int
	userTokensStartAt  time.Time
	userTokensEndAt    time.Time
	qualifiedStartAt   time.Time
	qualifiedEndAt     time.Time
	upsertEntryDate    time.Time
	publicWinners      []LotteryPublicWinner
	recentLimitArg     int
	participantCount    int64
	participantStartArg time.Time
	participantEndArg   time.Time

	designationCandidates []LotteryDesignationCandidate
	designations          []LotteryWinnerDesignation
	replacedDesignations  []LotteryDesignationInput
	replaceDesignateCalls int
}

func (r *lotteryServiceRepoStub) ListLotteryCampaigns(context.Context, int, int) ([]LotteryCampaign, int64, error) {
	return []LotteryCampaign{r.campaign}, 1, nil
}
func (r *lotteryServiceRepoStub) CreateLotteryCampaign(context.Context, LotteryCampaignInput) (*LotteryCampaign, error) {
	return &r.campaign, nil
}
func (r *lotteryServiceRepoStub) UpdateLotteryCampaign(context.Context, int64, LotteryCampaignInput) (*LotteryCampaign, error) {
	return &r.campaign, nil
}
func (r *lotteryServiceRepoStub) GetLotteryCampaign(context.Context, int64) (*LotteryCampaign, error) {
	return &r.campaign, nil
}
func (r *lotteryServiceRepoStub) GetActiveLotteryCampaign(context.Context, time.Time) (*LotteryCampaign, error) {
	return &r.campaign, nil
}
func (r *lotteryServiceRepoStub) UpdateLotteryCampaignStatus(context.Context, int64, string, *int64) (*LotteryCampaign, error) {
	return &r.campaign, nil
}
func (r *lotteryServiceRepoStub) SetFeaturedLotteryCampaign(context.Context, int64, *int64) (*LotteryCampaign, error) {
	r.featureCalls++
	r.campaign.IsFeatured = true
	return &r.campaign, nil
}
func (r *lotteryServiceRepoStub) DeleteLotteryCampaign(context.Context, int64) error {
	r.deleteCalls++
	return nil
}
func (r *lotteryServiceRepoStub) ListPublishedLotteryCampaigns(context.Context) ([]LotteryCampaign, error) {
	return []LotteryCampaign{r.campaign}, nil
}
func (r *lotteryServiceRepoStub) GetLotteryUserTokens(_ context.Context, _ int64, startAt, endAt time.Time) (int64, error) {
	r.userTokensStartAt = startAt
	r.userTokensEndAt = endAt
	return r.tokens, nil
}
func (r *lotteryServiceRepoStub) ListLotteryQualifiedUsage(_ context.Context, startAt, endAt time.Time, _ int64) ([]LotteryQualifiedUsage, error) {
	r.qualifiedStartAt = startAt
	r.qualifiedEndAt = endAt
	return r.qualified, nil
}
func (r *lotteryServiceRepoStub) UpsertLotteryEntry(_ context.Context, _ LotteryCampaign, _ int64, entryDate time.Time, _ int64, entryCount int, _ bool) (*LotteryEntry, error) {
	r.upsertCalls++
	r.upsertEntryDate = entryDate
	return &LotteryEntry{Status: LotteryEntryEnrolled, EntryCount: entryCount}, nil
}
func (r *lotteryServiceRepoStub) GetLotteryEntry(context.Context, int64, int64, time.Time) (*LotteryEntry, error) {
	return &LotteryEntry{}, nil
}
func (r *lotteryServiceRepoStub) ListLotteryDrawCandidates(context.Context, int64, time.Time) ([]LotteryDrawCandidate, error) {
	return r.candidates, nil
}
func (r *lotteryServiceRepoStub) CountLotteryQualifiedUsers(_ context.Context, startAt, endAt time.Time, _ int64) (int64, error) {
	r.participantStartArg = startAt
	r.participantEndArg = endAt
	return r.participantCount, nil
}
func (r *lotteryServiceRepoStub) GetLotteryDrawBatch(context.Context, int64, time.Time) (*LotteryDrawBatch, error) {
	if r.batch == nil {
		return nil, ErrLotteryCampaignNotFound
	}
	return r.batch, nil
}
func (r *lotteryServiceRepoStub) CreateLotteryDrawBatch(_ context.Context, campaignID int64, drawDate, scheduledDrawAt time.Time, triggerType string, operatorID *int64) (*LotteryDrawBatch, error) {
	r.createBatchCalls++
	if r.batch == nil {
		r.batch = &LotteryDrawBatch{ID: 9, CampaignID: campaignID, DrawDate: drawDate, ScheduledDrawAt: scheduledDrawAt, BatchNo: "lottery-7-20260708", Status: lotteryDrawStatusRunning, TriggerType: triggerType, OperatorID: operatorID, CreatedAt: scheduledDrawAt}
	}
	return r.batch, nil
}
func (r *lotteryServiceRepoStub) CreateLotteryWinners(_ context.Context, batch LotteryDrawBatch, campaign LotteryCampaign, winners []LotteryWinner) error {
	r.createWinnersCalls++
	if len(r.winners) == 0 {
		for i := range winners {
			winners[i].ID = int64(i + 1)
			winners[i].Status = "pending"
			winners[i].BatchID = batch.ID
			winners[i].CampaignID = campaign.ID
			r.winners = append(r.winners, winners[i])
		}
	}
	return nil
}
func (r *lotteryServiceRepoStub) GrantLotteryWinnerBalance(_ context.Context, winnerID int64, _ string) (*LotteryBalanceGrantResult, bool, error) {
	for i := range r.winners {
		if r.winners[i].ID != winnerID {
			continue
		}
		if r.winners[i].Status == lotteryWinnerStatusSuccess {
			return nil, false, nil
		}
		r.grantCalls++
		r.winners[i].Status = lotteryWinnerStatusSuccess
		return &LotteryBalanceGrantResult{WinnerID: winnerID, UserID: r.winners[i].UserID, BalanceAfter: float64(r.winners[i].RewardAmountCents) / 100}, true, nil
	}
	return nil, false, ErrLotteryCampaignNotFound
}
func (r *lotteryServiceRepoStub) MarkLotteryWinnerSuccess(context.Context, int64, float64, float64) error {
	return nil
}
func (r *lotteryServiceRepoStub) MarkLotteryWinnerFailed(context.Context, int64, string) error {
	return nil
}
func (r *lotteryServiceRepoStub) UpdateLotteryDrawBatchSummary(context.Context, int64) (*LotteryDrawBatch, error) {
	if r.batch == nil {
		return nil, ErrLotteryCampaignNotFound
	}
	success := 0
	for _, winner := range r.winners {
		if winner.Status == lotteryWinnerStatusSuccess {
			success++
		}
	}
	switch {
	case len(r.winners) == 0 || success == len(r.winners):
		r.batch.Status = lotteryDrawStatusSuccess
	case success > 0:
		r.batch.Status = lotteryDrawStatusPartial
	default:
		r.batch.Status = lotteryDrawStatusFailed
	}
	return r.batch, nil
}
func (r *lotteryServiceRepoStub) ListLotteryDrawBatches(context.Context, int64, int, int) ([]LotteryDrawBatch, int64, error) {
	if r.batch == nil {
		return nil, 0, nil
	}
	return []LotteryDrawBatch{*r.batch}, 1, nil
}
func (r *lotteryServiceRepoStub) ListLotteryWinners(context.Context, int64, *int64, *int64) ([]LotteryWinner, error) {
	out := append([]LotteryWinner(nil), r.winners...)
	return out, nil
}
func (r *lotteryServiceRepoStub) ListRecentPublicLotteryWinners(_ context.Context, _ int64, limit int) ([]LotteryPublicWinner, error) {
	r.recentLimitArg = limit
	return append([]LotteryPublicWinner(nil), r.publicWinners...), nil
}
func (r *lotteryServiceRepoStub) ListLotteryDesignationCandidates(context.Context, int64, time.Time) ([]LotteryDesignationCandidate, error) {
	return append([]LotteryDesignationCandidate(nil), r.designationCandidates...), nil
}
func (r *lotteryServiceRepoStub) GetLotteryDesignations(context.Context, int64, time.Time) ([]LotteryWinnerDesignation, error) {
	return append([]LotteryWinnerDesignation(nil), r.designations...), nil
}
func (r *lotteryServiceRepoStub) ReplaceLotteryDesignations(_ context.Context, _ int64, _ time.Time, _ *int64, inputs []LotteryDesignationInput) error {
	r.replaceDesignateCalls++
	r.replacedDesignations = append([]LotteryDesignationInput(nil), inputs...)
	return nil
}

func lotteryPtrTime(value time.Time) *time.Time {
	return &value
}

func lotteryValidInput(start time.Time) LotteryCampaignInput {
	drawAt := start.Add(2 * time.Hour)
	return LotteryCampaignInput{
		Name:              "Token 抽奖",
		ParticipationMode: LotteryParticipationAuto,
		DrawScheduleType:  LotteryDrawSingle,
		PrizeMode:         LotteryPrizeSingle,
		EntryMode:         LotteryEntryDailyOnce,
		ThresholdTokens:   100,
		MaxEntriesPerUser: 1,
		StartAt:           start,
		EndAt:             start.Add(24 * time.Hour),
		DrawAt:            &drawAt,
		PrizeTiers:        []LotteryPrizeTierInput{{TierName: "一等奖", WinnerCount: 1, RewardAmountCents: 1000}},
	}
}

type lotteryBalanceGrantCacheStub struct {
	invalidatedUserIDs []int64
}

func (s *lotteryBalanceGrantCacheStub) GrantUserBalances(context.Context, []BalanceGrantInput, string) ([]BalanceGrantResult, error) {
	return nil, nil
}

func (s *lotteryBalanceGrantCacheStub) invalidateBalanceGrantCaches(_ context.Context, userIDs []int64) {
	s.invalidatedUserIDs = append(s.invalidatedUserIDs, userIDs...)
}

func TestSelectLotteryWinnersWithDesignationsHybridFill(t *testing.T) {
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.Local)
	campaign := LotteryCampaign{
		ID: 7,
		PrizeTiers: []LotteryPrizeTier{
			{ID: 1, CampaignID: 7, TierName: "一等奖", WinnerCount: 2, RewardAmountCents: 1000, SortOrder: 1},
			{ID: 2, CampaignID: 7, TierName: "二等奖", WinnerCount: 1, RewardAmountCents: 500, SortOrder: 2},
		},
	}
	batch := LotteryDrawBatch{ID: 9, CampaignID: 7, DrawDate: drawDate}
	candidates := []LotteryDrawCandidate{
		{UserID: 1, EntryDate: drawDate, EntryCount: 5},
		{UserID: 2, EntryDate: drawDate, EntryCount: 3},
		{UserID: 3, EntryDate: drawDate, EntryCount: 1},
	}
	// user 2 预置为一等奖：应必中一等奖，且不进入随机池
	designations := []LotteryWinnerDesignation{{UserID: 2, PrizeTierID: 1}}

	winners, err := selectLotteryWinnersWithDesignations(campaign, batch, candidates, designations)
	if err != nil {
		t.Fatalf("select winners: %v", err)
	}
	if len(winners) != 3 {
		t.Fatalf("winners = %d, want 3 (2 first-tier + 1 second-tier)", len(winners))
	}
	var user2Tier int64
	seen := map[int64]bool{}
	for _, w := range winners {
		if seen[w.UserID] {
			t.Fatalf("user %d won more than once", w.UserID)
		}
		seen[w.UserID] = true
		if w.UserID == 2 {
			user2Tier = w.PrizeTierID
		}
	}
	if user2Tier != 1 {
		t.Fatalf("designated user 2 tier = %d, want 1 (first prize)", user2Tier)
	}
}

func TestSelectLotteryWinnersDesignationToUnknownTierIgnored(t *testing.T) {
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.Local)
	campaign := LotteryCampaign{
		ID: 7,
		PrizeTiers: []LotteryPrizeTier{
			{ID: 1, CampaignID: 7, TierName: "一等奖", WinnerCount: 2, RewardAmountCents: 1000, SortOrder: 1},
		},
	}
	batch := LotteryDrawBatch{ID: 9, CampaignID: 7, DrawDate: drawDate}
	candidates := []LotteryDrawCandidate{
		{UserID: 1, EntryDate: drawDate, EntryCount: 1},
		{UserID: 2, EntryDate: drawDate, EntryCount: 1},
	}
	// 预置指向不存在的奖项 99：应被忽略，user 1 仍可能随机中奖
	designations := []LotteryWinnerDesignation{{UserID: 1, PrizeTierID: 99}}

	winners, err := selectLotteryWinnersWithDesignations(campaign, batch, candidates, designations)
	if err != nil {
		t.Fatalf("select winners: %v", err)
	}
	// 一等奖 2 名额、2 候选人：两人都应中奖（陈旧预置不应把 user 1 踢出随机池）
	if len(winners) != 2 {
		t.Fatalf("winners = %d, want 2 (stale designation must not exclude candidate)", len(winners))
	}
	seen := map[int64]bool{}
	for _, w := range winners {
		seen[w.UserID] = true
	}
	if !seen[1] || !seen[2] {
		t.Fatalf("both candidates should win, got %v", seen)
	}
}

func lotteryDesignationRepo() *lotteryServiceRepoStub {
	return &lotteryServiceRepoStub{
		campaign: LotteryCampaign{
			ID:               7,
			Status:           LotteryStatusPublished,
			DrawScheduleType: LotteryDrawDaily,
			PrizeTiers: []LotteryPrizeTier{
				{ID: 1, CampaignID: 7, TierName: "一等奖", WinnerCount: 1, RewardAmountCents: 1000, SortOrder: 1},
				{ID: 2, CampaignID: 7, TierName: "二等奖", WinnerCount: 2, RewardAmountCents: 500, SortOrder: 2},
			},
		},
		designationCandidates: []LotteryDesignationCandidate{
			{UserID: 11, EntryCount: 3}, {UserID: 12, EntryCount: 2}, {UserID: 13, EntryCount: 1},
		},
	}
}

func TestReplaceDesignationsHappyPath(t *testing.T) {
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.Local)
	repo := lotteryDesignationRepo()
	svc := NewLotteryCampaignService(repo, nil)
	err := svc.ReplaceDesignations(context.Background(), 7, drawDate, nil, []LotteryDesignationInput{
		{UserID: 11, PrizeTierID: 1}, {UserID: 12, PrizeTierID: 2},
	})
	if err != nil {
		t.Fatalf("replace designations: %v", err)
	}
	if repo.replaceDesignateCalls != 1 {
		t.Fatalf("replace calls = %d, want 1", repo.replaceDesignateCalls)
	}
	if len(repo.replacedDesignations) != 2 {
		t.Fatalf("replaced = %d, want 2", len(repo.replacedDesignations))
	}
}

func TestReplaceDesignationsRejectsNotEnrolled(t *testing.T) {
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.Local)
	repo := lotteryDesignationRepo()
	svc := NewLotteryCampaignService(repo, nil)
	err := svc.ReplaceDesignations(context.Background(), 7, drawDate, nil, []LotteryDesignationInput{
		{UserID: 999, PrizeTierID: 1},
	})
	if !errors.Is(err, ErrLotteryDesignationNotEnroll) {
		t.Fatalf("err = %v, want ErrLotteryDesignationNotEnroll", err)
	}
	if repo.replaceDesignateCalls != 0 {
		t.Fatalf("must not persist on validation failure, got %d calls", repo.replaceDesignateCalls)
	}
}
func TestReplaceDesignationsRejectsTierFull(t *testing.T) {
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.Local)
	repo := lotteryDesignationRepo()
	svc := NewLotteryCampaignService(repo, nil)
	// 一等奖仅 1 名额，指定 2 人应被拒
	err := svc.ReplaceDesignations(context.Background(), 7, drawDate, nil, []LotteryDesignationInput{
		{UserID: 11, PrizeTierID: 1}, {UserID: 12, PrizeTierID: 1},
	})
	if !errors.Is(err, ErrLotteryDesignationTierFull) {
		t.Fatalf("err = %v, want ErrLotteryDesignationTierFull", err)
	}
	if repo.replaceDesignateCalls != 0 {
		t.Fatalf("must not persist, got %d calls", repo.replaceDesignateCalls)
	}
}

func TestReplaceDesignationsRejectsUnknownTier(t *testing.T) {
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.Local)
	repo := lotteryDesignationRepo()
	svc := NewLotteryCampaignService(repo, nil)
	err := svc.ReplaceDesignations(context.Background(), 7, drawDate, nil, []LotteryDesignationInput{
		{UserID: 11, PrizeTierID: 999},
	})
	if !errors.Is(err, ErrLotteryDesignationTierInval) {
		t.Fatalf("err = %v, want ErrLotteryDesignationTierInval", err)
	}
}

func TestReplaceDesignationsRejectsDuplicateUser(t *testing.T) {
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.Local)
	repo := lotteryDesignationRepo()
	svc := NewLotteryCampaignService(repo, nil)
	err := svc.ReplaceDesignations(context.Background(), 7, drawDate, nil, []LotteryDesignationInput{
		{UserID: 11, PrizeTierID: 1}, {UserID: 11, PrizeTierID: 2},
	})
	if !errors.Is(err, ErrLotteryInvalidConfig) {
		t.Fatalf("err = %v, want ErrLotteryInvalidConfig for duplicate user", err)
	}
}

func TestReplaceDesignationsRejectsWhenBatchLocked(t *testing.T) {
	drawDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.Local)
	repo := lotteryDesignationRepo()
	repo.batch = &LotteryDrawBatch{ID: 9, CampaignID: 7, DrawDate: drawDate, Status: lotteryDrawStatusSuccess}
	svc := NewLotteryCampaignService(repo, nil)
	err := svc.ReplaceDesignations(context.Background(), 7, drawDate, nil, []LotteryDesignationInput{
		{UserID: 11, PrizeTierID: 1},
	})
	if !errors.Is(err, ErrLotteryDesignationLocked) {
		t.Fatalf("err = %v, want ErrLotteryDesignationLocked", err)
	}
}
