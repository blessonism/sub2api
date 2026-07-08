package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/robfig/cron/v3"
)

const (
	LotteryStatusDraft     = "draft"
	LotteryStatusPublished = "published"
	LotteryStatusCancelled = "cancelled"
	LotteryStatusArchived  = "archived"

	LotteryParticipationAuto   = "auto"
	LotteryParticipationManual = "manual"
	LotteryDrawSingle          = "single"
	LotteryDrawDaily           = "daily"
	LotteryPrizeSingle         = "single"
	LotteryPrizeMulti          = "multi"
	LotteryEntryDailyOnce      = "daily_once"
	LotteryEntryStepped        = "stepped"
	LotteryEntryEligible       = "eligible"
	LotteryEntryEnrolled       = "enrolled"
	lotteryDrawStatusSuccess   = "success"
	lotteryDrawStatusPartial   = "partial_success"
	lotteryDrawStatusFailed    = "failed"
	lotteryDrawStatusRunning   = "processing"
	lotteryWinnerStatusSuccess = "success"
)

var (
	ErrLotteryCampaignNotFound = infraerrors.NotFound("LOTTERY_CAMPAIGN_NOT_FOUND", "lottery campaign not found")
	ErrLotteryInvalidConfig    = infraerrors.BadRequest("LOTTERY_INVALID_CONFIG", "invalid lottery campaign config")
	ErrLotteryNotEligible      = infraerrors.BadRequest("LOTTERY_NOT_ELIGIBLE", "token threshold is not reached")
	ErrLotteryAlreadyDrawn     = infraerrors.Conflict("LOTTERY_ALREADY_DRAWN", "lottery draw already completed")
	ErrLotteryNotPublished     = infraerrors.Conflict("LOTTERY_NOT_PUBLISHED", "lottery campaign must be published before syncing entries or drawing")
	ErrLotteryDrawNotDue       = infraerrors.Conflict("LOTTERY_DRAW_NOT_DUE", "lottery draw time has not arrived")
	ErrLotteryEntriesClosed    = infraerrors.Conflict("LOTTERY_ENTRIES_CLOSED", "lottery campaign is not accepting entries")
)

const lotteryProcessingRecoveryAfter = 10 * time.Minute

type LotteryCampaign struct {
	ID                int64              `json:"id"`
	Name              string             `json:"name"`
	Description       string             `json:"description"`
	RulesText         string             `json:"rules_text"`
	Status            string             `json:"status"`
	ParticipationMode string             `json:"participation_mode"`
	DrawScheduleType  string             `json:"draw_schedule_type"`
	PrizeMode         string             `json:"prize_mode"`
	EntryMode         string             `json:"entry_mode"`
	ThresholdTokens   int64              `json:"threshold_tokens"`
	EntryStepTokens   int64              `json:"entry_step_tokens"`
	MaxEntriesPerUser int                `json:"max_entries_per_user"`
	StartAt           time.Time          `json:"start_at"`
	EndAt             time.Time          `json:"end_at"`
	DrawAt            *time.Time         `json:"draw_at,omitempty"`
	DailyDrawTime     string             `json:"daily_draw_time"`
	CreatedBy         *int64             `json:"created_by,omitempty"`
	UpdatedBy         *int64             `json:"updated_by,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
	PrizeTiers        []LotteryPrizeTier `json:"prize_tiers,omitempty"`
}

type LotteryPrizeTier struct {
	ID                int64     `json:"id"`
	CampaignID        int64     `json:"campaign_id"`
	TierName          string    `json:"tier_name"`
	WinnerCount       int       `json:"winner_count"`
	RewardAmountCents int64     `json:"reward_amount_cents"`
	SortOrder         int       `json:"sort_order"`
	CreatedAt         time.Time `json:"created_at"`
}

type LotteryEntry struct {
	ID         int64      `json:"id"`
	CampaignID int64      `json:"campaign_id"`
	UserID     int64      `json:"user_id"`
	EntryDate  time.Time  `json:"entry_date"`
	Tokens     int64      `json:"tokens"`
	EntryCount int        `json:"entry_count"`
	Status     string     `json:"status"`
	EnrolledAt *time.Time `json:"enrolled_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type LotteryDrawBatch struct {
	ID              int64      `json:"id"`
	CampaignID      int64      `json:"campaign_id"`
	DrawDate        time.Time  `json:"draw_date"`
	ScheduledDrawAt time.Time  `json:"scheduled_draw_at"`
	BatchNo         string     `json:"batch_no"`
	Status          string     `json:"status"`
	TriggerType     string     `json:"trigger_type"`
	OperatorID      *int64     `json:"operator_id,omitempty"`
	TotalEntries    int        `json:"total_entries"`
	TotalWinners    int        `json:"total_winners"`
	ErrorMessage    string     `json:"error_message"`
	CreatedAt       time.Time  `json:"created_at"`
	DrawnAt         *time.Time `json:"drawn_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

type LotteryWinner struct {
	ID                    int64      `json:"id"`
	BatchID               int64      `json:"batch_id"`
	CampaignID            int64      `json:"campaign_id"`
	UserID                int64      `json:"user_id"`
	PrizeTierID           int64      `json:"prize_tier_id"`
	PrizeName             string     `json:"prize_name,omitempty"`
	EntryDate             time.Time  `json:"entry_date"`
	RewardAmountCents     int64      `json:"reward_amount_cents"`
	Status                string     `json:"status"`
	BalanceBeforeSnapshot *float64   `json:"balance_before_snapshot,omitempty"`
	BalanceAfterSnapshot  *float64   `json:"balance_after_snapshot,omitempty"`
	IdempotencyKey        string     `json:"idempotency_key"`
	ErrorMessage          string     `json:"error_message"`
	CreatedAt             time.Time  `json:"created_at"`
	ProcessedAt           *time.Time `json:"processed_at,omitempty"`
}

type LotteryBalanceGrantResult struct {
	WinnerID      int64
	UserID        int64
	BalanceBefore float64
	BalanceAfter  float64
}

type LotteryCampaignInput struct {
	Name              string
	Description       string
	RulesText         string
	ParticipationMode string
	DrawScheduleType  string
	PrizeMode         string
	EntryMode         string
	ThresholdTokens   int64
	EntryStepTokens   int64
	MaxEntriesPerUser int
	StartAt           time.Time
	EndAt             time.Time
	DrawAt            *time.Time
	DailyDrawTime     string
	PrizeTiers        []LotteryPrizeTierInput
	OperatorID        *int64
}

type LotteryPrizeTierInput struct {
	TierName          string `json:"tier_name"`
	WinnerCount       int    `json:"winner_count"`
	RewardAmountCents int64  `json:"reward_amount_cents"`
	SortOrder         int    `json:"sort_order"`
}

type LotteryQualifiedUsage struct {
	UserID int64
	Tokens int64
}

type LotteryDrawCandidate struct {
	UserID     int64
	EntryDate  time.Time
	Tokens     int64
	EntryCount int
}

type LotteryMyData struct {
	Campaign        *LotteryCampaign `json:"campaign"`
	TodayTokens     int64            `json:"today_tokens"`
	ThresholdTokens int64            `json:"threshold_tokens"`
	EntryCount      int              `json:"entry_count"`
	EntryStatus     string           `json:"entry_status"`
	NextDrawAt      *time.Time       `json:"next_draw_at,omitempty"`
	Winners         []LotteryWinner  `json:"winners"`
}

type LotteryCampaignRepository interface {
	ListLotteryCampaigns(ctx context.Context, page, pageSize int) ([]LotteryCampaign, int64, error)
	CreateLotteryCampaign(ctx context.Context, input LotteryCampaignInput) (*LotteryCampaign, error)
	UpdateLotteryCampaign(ctx context.Context, id int64, input LotteryCampaignInput) (*LotteryCampaign, error)
	GetLotteryCampaign(ctx context.Context, id int64) (*LotteryCampaign, error)
	GetActiveLotteryCampaign(ctx context.Context, now time.Time) (*LotteryCampaign, error)
	UpdateLotteryCampaignStatus(ctx context.Context, id int64, status string, operatorID *int64) (*LotteryCampaign, error)
	ListPublishedLotteryCampaigns(ctx context.Context) ([]LotteryCampaign, error)
	GetLotteryUserTokens(ctx context.Context, userID int64, startAt, endAt time.Time) (int64, error)
	ListLotteryQualifiedUsage(ctx context.Context, startAt, endAt time.Time, minTokens int64) ([]LotteryQualifiedUsage, error)
	UpsertLotteryEntry(ctx context.Context, campaign LotteryCampaign, userID int64, entryDate time.Time, tokens int64, entryCount int, enrolled bool) (*LotteryEntry, error)
	GetLotteryEntry(ctx context.Context, campaignID, userID int64, entryDate time.Time) (*LotteryEntry, error)
	ListLotteryDrawCandidates(ctx context.Context, campaignID int64, entryDate time.Time) ([]LotteryDrawCandidate, error)
	GetLotteryDrawBatch(ctx context.Context, campaignID int64, drawDate time.Time) (*LotteryDrawBatch, error)
	CreateLotteryDrawBatch(ctx context.Context, campaignID int64, drawDate, scheduledDrawAt time.Time, triggerType string, operatorID *int64) (*LotteryDrawBatch, error)
	CreateLotteryWinners(ctx context.Context, batch LotteryDrawBatch, campaign LotteryCampaign, winners []LotteryWinner) error
	GrantLotteryWinnerBalance(ctx context.Context, winnerID int64, notes string) (*LotteryBalanceGrantResult, bool, error)
	MarkLotteryWinnerSuccess(ctx context.Context, winnerID int64, before, after float64) error
	MarkLotteryWinnerFailed(ctx context.Context, winnerID int64, message string) error
	UpdateLotteryDrawBatchSummary(ctx context.Context, batchID int64) (*LotteryDrawBatch, error)
	ListLotteryDrawBatches(ctx context.Context, campaignID int64, page, pageSize int) ([]LotteryDrawBatch, int64, error)
	ListLotteryWinners(ctx context.Context, campaignID int64, batchID *int64, userID *int64) ([]LotteryWinner, error)
}

type LotteryCampaignService struct {
	repo         LotteryCampaignRepository
	balanceGrant CampaignBalanceGrantService
}

type lotteryBalanceGrantCacheInvalidator interface {
	invalidateBalanceGrantCaches(ctx context.Context, userIDs []int64)
}

func NewLotteryCampaignService(repo LotteryCampaignRepository, balanceGrant CampaignBalanceGrantService) *LotteryCampaignService {
	return &LotteryCampaignService{repo: repo, balanceGrant: balanceGrant}
}

func (s *LotteryCampaignService) List(ctx context.Context, page, pageSize int) ([]LotteryCampaign, int64, error) {
	return s.repo.ListLotteryCampaigns(ctx, page, pageSize)
}

func (s *LotteryCampaignService) Create(ctx context.Context, input LotteryCampaignInput) (*LotteryCampaign, error) {
	normalizeLotteryInput(&input)
	if err := validateLotteryInput(input); err != nil {
		return nil, err
	}
	return s.repo.CreateLotteryCampaign(ctx, input)
}

func (s *LotteryCampaignService) Update(ctx context.Context, id int64, input LotteryCampaignInput) (*LotteryCampaign, error) {
	normalizeLotteryInput(&input)
	if err := validateLotteryInput(input); err != nil {
		return nil, err
	}
	return s.repo.UpdateLotteryCampaign(ctx, id, input)
}

func (s *LotteryCampaignService) Get(ctx context.Context, id int64) (*LotteryCampaign, error) {
	return s.repo.GetLotteryCampaign(ctx, id)
}

func (s *LotteryCampaignService) Publish(ctx context.Context, id int64, operatorID *int64) (*LotteryCampaign, error) {
	return s.repo.UpdateLotteryCampaignStatus(ctx, id, LotteryStatusPublished, operatorID)
}

func (s *LotteryCampaignService) Cancel(ctx context.Context, id int64, operatorID *int64) (*LotteryCampaign, error) {
	return s.repo.UpdateLotteryCampaignStatus(ctx, id, LotteryStatusCancelled, operatorID)
}

func (s *LotteryCampaignService) Active(ctx context.Context, now time.Time) (*LotteryCampaign, error) {
	return s.repo.GetActiveLotteryCampaign(ctx, now)
}

func (s *LotteryCampaignService) MyData(ctx context.Context, campaignID, userID int64, now time.Time) (*LotteryMyData, error) {
	campaign, err := s.repo.GetLotteryCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if !lotteryCampaignVisible(*campaign, now) {
		return nil, ErrLotteryCampaignNotFound
	}
	drawDate, windowStart, windowEnd, nextDrawAt := lotteryCurrentWindow(*campaign, now)
	tokens, err := s.repo.GetLotteryUserTokens(ctx, userID, windowStart, lotteryMinTime(now, windowEnd))
	if err != nil {
		return nil, err
	}
	entryCount := lotteryEntryCount(*campaign, tokens)
	var status string
	if entryCount > 0 && lotteryCampaignAcceptsEntries(*campaign, now) {
		enrolled := campaign.ParticipationMode == LotteryParticipationAuto
		entry, err := s.repo.UpsertLotteryEntry(ctx, *campaign, userID, drawDate, tokens, entryCount, enrolled)
		if err != nil {
			return nil, err
		}
		status = entry.Status
		entryCount = entry.EntryCount
	} else {
		status = "not_eligible"
		entryCount = 0
	}
	winners, err := s.repo.ListLotteryWinners(ctx, campaign.ID, nil, &userID)
	if err != nil {
		return nil, err
	}
	return &LotteryMyData{
		Campaign:        campaign,
		TodayTokens:     tokens,
		ThresholdTokens: campaign.ThresholdTokens,
		EntryCount:      entryCount,
		EntryStatus:     status,
		NextDrawAt:      nextDrawAt,
		Winners:         winners,
	}, nil
}

func (s *LotteryCampaignService) Enroll(ctx context.Context, campaignID, userID int64, now time.Time) (*LotteryEntry, error) {
	campaign, err := s.repo.GetLotteryCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if !lotteryCampaignVisible(*campaign, now) {
		return nil, ErrLotteryCampaignNotFound
	}
	if !lotteryCampaignAcceptsEntries(*campaign, now) {
		return nil, ErrLotteryEntriesClosed
	}
	drawDate, windowStart, windowEnd, _ := lotteryCurrentWindow(*campaign, now)
	tokens, err := s.repo.GetLotteryUserTokens(ctx, userID, windowStart, lotteryMinTime(now, windowEnd))
	if err != nil {
		return nil, err
	}
	entryCount := lotteryEntryCount(*campaign, tokens)
	if entryCount <= 0 {
		return nil, ErrLotteryNotEligible
	}
	return s.repo.UpsertLotteryEntry(ctx, *campaign, userID, drawDate, tokens, entryCount, true)
}

func (s *LotteryCampaignService) SyncEntries(ctx context.Context, campaignID int64, drawDate time.Time, now time.Time) (int, error) {
	campaign, err := s.repo.GetLotteryCampaign(ctx, campaignID)
	if err != nil {
		return 0, err
	}
	if campaign.Status != LotteryStatusPublished {
		return 0, ErrLotteryNotPublished
	}
	scheduledAt := lotteryScheduledAt(*campaign, drawDate)
	if scheduledAt.IsZero() {
		scheduledAt = now
	}
	windowStart := timezone.StartOfDay(scheduledAt)
	windowEnd := lotteryMinTime(scheduledAt, now)
	usages, err := s.repo.ListLotteryQualifiedUsage(ctx, windowStart, windowEnd, campaign.ThresholdTokens)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, usage := range usages {
		entryCount := lotteryEntryCount(*campaign, usage.Tokens)
		if entryCount <= 0 {
			continue
		}
		_, err := s.repo.UpsertLotteryEntry(ctx, *campaign, usage.UserID, drawDate, usage.Tokens, entryCount, campaign.ParticipationMode == LotteryParticipationAuto)
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *LotteryCampaignService) Draw(ctx context.Context, campaignID int64, drawDate time.Time, triggerType string, operatorID *int64, now time.Time) (*LotteryDrawBatch, error) {
	campaign, err := s.repo.GetLotteryCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if campaign.Status != LotteryStatusPublished {
		return nil, ErrLotteryNotPublished
	}
	drawDate = dateOnly(drawDate)
	if found, err := s.repo.GetLotteryDrawBatch(ctx, campaignID, drawDate); err == nil && found != nil {
		if !lotteryBatchCanResume(*found, now) {
			return found, nil
		}
	}
	scheduledAt := lotteryScheduledAt(*campaign, drawDate)
	if scheduledAt.IsZero() {
		scheduledAt = now
	}
	if now.Before(scheduledAt) {
		return nil, ErrLotteryDrawNotDue
	}
	if _, err := s.SyncEntries(ctx, campaignID, drawDate, scheduledAt); err != nil {
		return nil, err
	}
	batch, err := s.repo.CreateLotteryDrawBatch(ctx, campaignID, drawDate, scheduledAt, triggerType, operatorID)
	if err != nil {
		return nil, err
	}
	winners, err := s.repo.ListLotteryWinners(ctx, campaignID, &batch.ID, nil)
	if err != nil {
		return nil, err
	}
	if len(winners) == 0 {
		candidates, err := s.repo.ListLotteryDrawCandidates(ctx, campaignID, drawDate)
		if err != nil {
			return nil, err
		}
		selectedWinners, err := selectLotteryWinners(*campaign, *batch, candidates)
		if err != nil {
			return nil, err
		}
		if err := s.repo.CreateLotteryWinners(ctx, *batch, *campaign, selectedWinners); err != nil {
			return nil, err
		}
		winners, err = s.repo.ListLotteryWinners(ctx, campaignID, &batch.ID, nil)
		if err != nil {
			return nil, err
		}
	}
	for _, winner := range winners {
		if winner.Status == lotteryWinnerStatusSuccess {
			continue
		}
		note := fmt.Sprintf("Token 抽奖活动发放：%s / %s / %s", campaign.Name, batch.BatchNo, winner.PrizeName)
		grant, claimed, grantErr := s.repo.GrantLotteryWinnerBalance(ctx, winner.ID, note)
		if grantErr != nil {
			_ = s.repo.MarkLotteryWinnerFailed(ctx, winner.ID, grantErr.Error())
			continue
		}
		if !claimed || grant == nil {
			continue
		}
		s.invalidateLotteryBalanceGrantCaches(ctx, grant.UserID)
	}
	return s.repo.UpdateLotteryDrawBatchSummary(ctx, batch.ID)
}

func (s *LotteryCampaignService) invalidateLotteryBalanceGrantCaches(ctx context.Context, userID int64) {
	if s == nil || s.balanceGrant == nil || userID <= 0 {
		return
	}
	invalidator, ok := s.balanceGrant.(lotteryBalanceGrantCacheInvalidator)
	if !ok {
		return
	}
	invalidator.invalidateBalanceGrantCaches(ctx, []int64{userID})
}

func (s *LotteryCampaignService) ListBatches(ctx context.Context, campaignID int64, page, pageSize int) ([]LotteryDrawBatch, int64, error) {
	return s.repo.ListLotteryDrawBatches(ctx, campaignID, page, pageSize)
}

func (s *LotteryCampaignService) ListWinners(ctx context.Context, campaignID int64, batchID *int64) ([]LotteryWinner, error) {
	return s.repo.ListLotteryWinners(ctx, campaignID, batchID, nil)
}

func (s *LotteryCampaignService) RunDueDraws(ctx context.Context, now time.Time) {
	campaigns, err := s.repo.ListPublishedLotteryCampaigns(ctx)
	if err != nil {
		logger.LegacyPrintf("service.lottery_campaign", "[LotteryCampaign] list published failed: %v", err)
		return
	}
	for _, campaign := range campaigns {
		for _, due := range lotteryDueDates(campaign, now) {
			if _, err := s.Draw(ctx, campaign.ID, due, "scheduled", nil, now); err != nil && !errors.Is(err, ErrLotteryAlreadyDrawn) {
				logger.LegacyPrintf("service.lottery_campaign", "[LotteryCampaign] draw failed: campaign=%d date=%s err=%v", campaign.ID, due.Format("2006-01-02"), err)
			}
		}
	}
}

func validateLotteryInput(input LotteryCampaignInput) error {
	if strings.TrimSpace(input.Name) == "" || input.ThresholdTokens <= 0 || input.EndAt.Before(input.StartAt) || input.EndAt.Equal(input.StartAt) {
		return ErrLotteryInvalidConfig
	}
	if input.ParticipationMode != LotteryParticipationAuto && input.ParticipationMode != LotteryParticipationManual {
		return ErrLotteryInvalidConfig
	}
	if input.DrawScheduleType != LotteryDrawSingle && input.DrawScheduleType != LotteryDrawDaily {
		return ErrLotteryInvalidConfig
	}
	if input.PrizeMode != LotteryPrizeSingle && input.PrizeMode != LotteryPrizeMulti {
		return ErrLotteryInvalidConfig
	}
	if input.EntryMode != LotteryEntryDailyOnce && input.EntryMode != LotteryEntryStepped {
		return ErrLotteryInvalidConfig
	}
	if input.EntryMode == LotteryEntryStepped && (input.EntryStepTokens <= 0 || input.MaxEntriesPerUser <= 1) {
		return ErrLotteryInvalidConfig
	}
	if input.DrawScheduleType == LotteryDrawSingle && input.DrawAt == nil {
		return ErrLotteryInvalidConfig
	}
	if input.DrawScheduleType == LotteryDrawDaily {
		if _, _, err := parseLotteryDailyDrawTime(input.DailyDrawTime); err != nil {
			return ErrLotteryInvalidConfig
		}
	}
	if len(input.PrizeTiers) == 0 {
		return ErrLotteryInvalidConfig
	}
	if input.PrizeMode == LotteryPrizeSingle && len(input.PrizeTiers) != 1 {
		return ErrLotteryInvalidConfig
	}
	for _, tier := range input.PrizeTiers {
		if strings.TrimSpace(tier.TierName) == "" || tier.WinnerCount <= 0 || tier.RewardAmountCents <= 0 {
			return ErrLotteryInvalidConfig
		}
	}
	return nil
}

func normalizeLotteryInput(input *LotteryCampaignInput) {
	if input == nil {
		return
	}
	if input.EntryMode == LotteryEntryDailyOnce {
		input.EntryStepTokens = 0
		input.MaxEntriesPerUser = 1
	}
	if input.EntryMode == LotteryEntryStepped && input.MaxEntriesPerUser <= 0 {
		input.MaxEntriesPerUser = 1
	}
}

func lotteryEntryCount(campaign LotteryCampaign, tokens int64) int {
	if tokens < campaign.ThresholdTokens {
		return 0
	}
	if campaign.EntryMode == LotteryEntryDailyOnce {
		return 1
	}
	step := campaign.EntryStepTokens
	if step <= 0 {
		return 1
	}
	count := 1 + int((tokens-campaign.ThresholdTokens)/step)
	if campaign.MaxEntriesPerUser > 0 && count > campaign.MaxEntriesPerUser {
		return campaign.MaxEntriesPerUser
	}
	if count < 1 {
		return 1
	}
	return count
}

func lotteryCurrentWindow(campaign LotteryCampaign, now time.Time) (time.Time, time.Time, time.Time, *time.Time) {
	locNow := now.In(timezone.Location())
	drawDate := dateOnly(locNow)
	next := lotteryScheduledAt(campaign, drawDate)
	if campaign.DrawScheduleType == LotteryDrawSingle && campaign.DrawAt != nil {
		drawDate = dateOnly(campaign.DrawAt.In(timezone.Location()))
		next = *campaign.DrawAt
	}
	if campaign.DrawScheduleType == LotteryDrawDaily && !next.IsZero() && locNow.After(next) {
		nextDay := drawDate.AddDate(0, 0, 1)
		nextAt := lotteryScheduledAt(campaign, nextDay)
		if nextAt.Before(campaign.EndAt) || nextAt.Equal(campaign.EndAt) {
			next = nextAt
			drawDate = nextDay
		}
	}
	start := timezone.StartOfDay(drawDate)
	end := next
	if end.IsZero() {
		end = lotteryMinTime(timezone.EndOfDay(drawDate), campaign.EndAt)
	}
	return drawDate, start, end, &next
}

func lotteryCampaignVisible(campaign LotteryCampaign, now time.Time) bool {
	if campaign.Status != LotteryStatusPublished {
		return false
	}
	locNow := now.In(timezone.Location())
	return !locNow.Before(campaign.StartAt.In(timezone.Location())) && !locNow.After(campaign.EndAt.In(timezone.Location()))
}

func lotteryCampaignAcceptsEntries(campaign LotteryCampaign, now time.Time) bool {
	if !lotteryCampaignVisible(campaign, now) {
		return false
	}
	_, _, windowEnd, _ := lotteryCurrentWindow(campaign, now)
	return !windowEnd.IsZero() && now.In(timezone.Location()).Before(windowEnd.In(timezone.Location()))
}

func lotteryScheduledAt(campaign LotteryCampaign, drawDate time.Time) time.Time {
	if campaign.DrawScheduleType == LotteryDrawSingle {
		if campaign.DrawAt == nil {
			return time.Time{}
		}
		return campaign.DrawAt.In(timezone.Location())
	}
	hour, minute, err := parseLotteryDailyDrawTime(campaign.DailyDrawTime)
	if err != nil {
		return time.Time{}
	}
	d := drawDate.In(timezone.Location())
	return time.Date(d.Year(), d.Month(), d.Day(), hour, minute, 0, 0, timezone.Location())
}

func lotteryDueDates(campaign LotteryCampaign, now time.Time) []time.Time {
	if campaign.Status != LotteryStatusPublished {
		return nil
	}
	now = now.In(timezone.Location())
	if campaign.DrawScheduleType == LotteryDrawSingle {
		if campaign.DrawAt != nil && !now.Before(campaign.DrawAt.In(timezone.Location())) {
			return []time.Time{dateOnly(campaign.DrawAt.In(timezone.Location()))}
		}
		return nil
	}
	start := dateOnly(campaign.StartAt.In(timezone.Location()))
	end := dateOnly(lotteryMinTime(now, campaign.EndAt.In(timezone.Location())))
	out := make([]time.Time, 0)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if len(out) > 370 {
			break
		}
		scheduled := lotteryScheduledAt(campaign, d)
		if scheduled.IsZero() || scheduled.Before(campaign.StartAt) || scheduled.After(campaign.EndAt) || now.Before(scheduled) {
			continue
		}
		out = append(out, d)
	}
	return out
}

func lotteryBatchCanResume(batch LotteryDrawBatch, now time.Time) bool {
	switch batch.Status {
	case lotteryDrawStatusSuccess:
		return false
	case lotteryDrawStatusPartial, lotteryDrawStatusFailed:
		return true
	case lotteryDrawStatusRunning:
		reference := batch.CreatedAt
		if batch.DrawnAt != nil {
			reference = *batch.DrawnAt
		}
		return now.Sub(reference) >= lotteryProcessingRecoveryAfter
	default:
		return true
	}
}

func selectLotteryWinners(campaign LotteryCampaign, batch LotteryDrawBatch, candidates []LotteryDrawCandidate) ([]LotteryWinner, error) {
	sort.Slice(campaign.PrizeTiers, func(i, j int) bool {
		if campaign.PrizeTiers[i].SortOrder == campaign.PrizeTiers[j].SortOrder {
			return campaign.PrizeTiers[i].ID < campaign.PrizeTiers[j].ID
		}
		return campaign.PrizeTiers[i].SortOrder < campaign.PrizeTiers[j].SortOrder
	})
	remaining := append([]LotteryDrawCandidate(nil), candidates...)
	winners := make([]LotteryWinner, 0)
	for _, tier := range campaign.PrizeTiers {
		for i := 0; i < tier.WinnerCount && len(remaining) > 0; i++ {
			idx, err := weightedLotteryIndex(remaining)
			if err != nil {
				return nil, err
			}
			selected := remaining[idx]
			winners = append(winners, LotteryWinner{
				BatchID:           batch.ID,
				CampaignID:        campaign.ID,
				UserID:            selected.UserID,
				PrizeTierID:       tier.ID,
				PrizeName:         tier.TierName,
				EntryDate:         selected.EntryDate,
				RewardAmountCents: tier.RewardAmountCents,
				IdempotencyKey:    fmt.Sprintf("lottery:%d:batch:%d:user:%d", campaign.ID, batch.ID, selected.UserID),
			})
			remaining = append(remaining[:idx], remaining[idx+1:]...)
		}
	}
	return winners, nil
}

func weightedLotteryIndex(candidates []LotteryDrawCandidate) (int, error) {
	var total int64
	for _, c := range candidates {
		if c.EntryCount > 0 {
			total += int64(c.EntryCount)
		}
	}
	if total <= 0 {
		return 0, ErrLotteryInvalidConfig
	}
	n, err := rand.Int(rand.Reader, big.NewInt(total))
	if err != nil {
		return 0, err
	}
	pick := n.Int64()
	var acc int64
	for i, c := range candidates {
		acc += int64(c.EntryCount)
		if pick < acc {
			return i, nil
		}
	}
	return len(candidates) - 1, nil
}

func parseLotteryDailyDrawTime(value string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid daily draw time")
	}
	var hour, minute int
	if _, err := fmt.Sscanf(value, "%02d:%02d", &hour, &minute); err != nil {
		return 0, 0, err
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid daily draw time")
	}
	return hour, minute, nil
}

func dateOnly(t time.Time) time.Time {
	t = t.In(timezone.Location())
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, timezone.Location())
}

func lotteryMinTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

type LotteryCampaignRunner struct {
	svc   *LotteryCampaignService
	cron  *cron.Cron
	start sync.Once
	stop  sync.Once
}

func NewLotteryCampaignRunner(svc *LotteryCampaignService) *LotteryCampaignRunner {
	return &LotteryCampaignRunner{svc: svc}
}

func (r *LotteryCampaignRunner) Start() {
	if r == nil || r.svc == nil {
		return
	}
	r.start.Do(func() {
		c := cron.New(cron.WithParser(scheduledTestCronParser), cron.WithLocation(timezone.Location()))
		if _, err := c.AddFunc("* * * * *", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			r.svc.RunDueDraws(ctx, timezone.Now())
		}); err != nil {
			logger.LegacyPrintf("service.lottery_campaign_runner", "[LotteryCampaignRunner] not started: %v", err)
			return
		}
		r.cron = c
		r.cron.Start()
		logger.LegacyPrintf("service.lottery_campaign_runner", "[LotteryCampaignRunner] started")
	})
}

func (r *LotteryCampaignRunner) Stop() {
	if r == nil {
		return
	}
	r.stop.Do(func() {
		if r.cron == nil {
			return
		}
		ctx := r.cron.Stop()
		select {
		case <-ctx.Done():
		case <-time.After(3 * time.Second):
			logger.LegacyPrintf("service.lottery_campaign_runner", "[LotteryCampaignRunner] cron stop timed out")
		}
	})
}
