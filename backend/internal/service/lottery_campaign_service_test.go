package service

import (
	"context"
	"errors"
	"testing"
	"time"
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

	winners, err := selectLotteryWinners(campaign, batch, candidates)
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
func (r *lotteryServiceRepoStub) ListPublishedLotteryCampaigns(context.Context) ([]LotteryCampaign, error) {
	return []LotteryCampaign{r.campaign}, nil
}
func (r *lotteryServiceRepoStub) GetLotteryUserTokens(context.Context, int64, time.Time, time.Time) (int64, error) {
	return r.tokens, nil
}
func (r *lotteryServiceRepoStub) ListLotteryQualifiedUsage(context.Context, time.Time, time.Time, int64) ([]LotteryQualifiedUsage, error) {
	return r.qualified, nil
}
func (r *lotteryServiceRepoStub) UpsertLotteryEntry(context.Context, LotteryCampaign, int64, time.Time, int64, int, bool) (*LotteryEntry, error) {
	r.upsertCalls++
	return &LotteryEntry{Status: LotteryEntryEnrolled, EntryCount: 1}, nil
}
func (r *lotteryServiceRepoStub) GetLotteryEntry(context.Context, int64, int64, time.Time) (*LotteryEntry, error) {
	return &LotteryEntry{}, nil
}
func (r *lotteryServiceRepoStub) ListLotteryDrawCandidates(context.Context, int64, time.Time) ([]LotteryDrawCandidate, error) {
	return r.candidates, nil
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

func lotteryPtrTime(value time.Time) *time.Time {
	return &value
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
