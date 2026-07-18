package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	CampaignStatusDraft         = "draft"
	CampaignStatusWarmup        = "warmup"
	CampaignStatusActive        = "active"
	CampaignStatusFrozen        = "frozen"
	CampaignStatusPaused        = "paused"
	CampaignStatusAuditing      = "auditing"
	CampaignStatusPublicizing   = "publicizing"
	CampaignStatusPendingPayout = "pending_payout"
	CampaignStatusPaid          = "paid"
	CampaignStatusCancelled     = "cancelled"
	CampaignStatusTerminated    = "terminated"

	CampaignConfigScopePublishSnapshot      = "publish_snapshot"
	CampaignConfigScopeThresholdAdjustment  = "threshold_adjustment"
	CampaignConfigScopeInjectionRateAdjust  = "injection_rate_adjustment"
	CampaignConfigScopePoolScopeAdjustment  = "pool_scope_adjustment"
	CampaignPoolInjectionScopeInviteesOnly  = "invitees_only"
	CampaignPoolInjectionScopeAllUsers      = "all_users"
	CampaignInviteStatusRegistered          = "registered"
	CampaignInviteStatusRechargeUnqualified = "recharge_unqualified"
	CampaignInviteStatusPendingAudit        = "pending_audit"
	CampaignInviteStatusEffective           = "effective"
	CampaignInviteStatusInvalid             = "invalid"
	CampaignInviteStatusRiskReview          = "risk_review"
	CampaignPoolStatusPending               = "pending"
	CampaignPoolStatusConfirmed             = "confirmed"
	CampaignPoolStatusDeducted              = "deducted"
	CampaignCalculationPreview              = "preview"
	CampaignCalculationFrozen               = "frozen"
	CampaignCalculationFinal                = "final"
)

var (
	ErrCampaignNotFound          = infraerrors.NotFound("CAMPAIGN_NOT_FOUND", "campaign not found")
	ErrCampaignInvalidConfig     = infraerrors.BadRequest("CAMPAIGN_INVALID_CONFIG", "invalid campaign config")
	ErrCampaignImmutableRule     = infraerrors.BadRequest("CAMPAIGN_IMMUTABLE_RULE", "campaign reward rules are frozen after publish")
	ErrCampaignDuplicateActive   = infraerrors.Conflict("CAMPAIGN_ACTIVE_EXISTS", "only one active campaign is allowed")
	ErrCampaignSettlementLocked  = infraerrors.Conflict("CAMPAIGN_SETTLEMENT_LOCKED", "campaign final settlement or payout batch already exists")
	ErrCampaignNoFinalSettlement = infraerrors.BadRequest("CAMPAIGN_NO_FINAL_SETTLEMENT", "final settlement is required before payout")
	ErrCampaignAlreadyPaid       = infraerrors.Conflict("CAMPAIGN_ALREADY_PAID", "campaign payout already completed")
	ErrCampaignDeleteBlocked     = infraerrors.Conflict("CAMPAIGN_DELETE_BLOCKED", "campaign has settlement or payout data and cannot be deleted")
)

var defaultCampaignRankWeights = []int64{30, 20, 15, 10, 8, 6, 4, 3, 2, 2}

type Campaign struct {
	ID                       int64      `json:"id"`
	Name                     string     `json:"name"`
	Description              string     `json:"description"`
	CoverURL                 string     `json:"cover_url"`
	RulesText                string     `json:"rules_text"`
	Status                   string     `json:"status"`
	WarmupStartAt            *time.Time `json:"warmup_start_at,omitempty"`
	StartAt                  time.Time  `json:"start_at"`
	EndAt                    time.Time  `json:"end_at"`
	AuditStartAt             *time.Time `json:"audit_start_at,omitempty"`
	AuditEndAt               *time.Time `json:"audit_end_at,omitempty"`
	PublicityStartAt         *time.Time `json:"publicity_start_at,omitempty"`
	PublicityEndAt           *time.Time `json:"publicity_end_at,omitempty"`
	PayoutDueAt              *time.Time `json:"payout_due_at,omitempty"`
	PublishedConfigVersionID *int64     `json:"published_config_version_id,omitempty"`
	CreatedBy                *int64     `json:"created_by,omitempty"`
	UpdatedBy                *int64     `json:"updated_by,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

type CampaignConfigVersion struct {
	ID                       int64           `json:"id"`
	CampaignID               int64           `json:"campaign_id"`
	Version                  int             `json:"version"`
	VersionScope             string          `json:"version_scope"`
	EffectiveAt              time.Time       `json:"effective_at"`
	RechargeThresholdCents   int64           `json:"recharge_threshold_cents"`
	AllowAccumulatedRecharge bool            `json:"allow_accumulated_recharge"`
	HistoricalInviteRatio    decimal.Decimal `json:"historical_invite_ratio"`
	PoolInjectionRate        decimal.Decimal `json:"pool_injection_rate"`
	PoolInjectionScope       string          `json:"pool_injection_scope"`
	RankPoolRatio            decimal.Decimal `json:"rank_pool_ratio"`
	ContributionPoolRatio    decimal.Decimal `json:"contribution_pool_ratio"`
	RankRewardCount          int             `json:"rank_reward_count"`
	RankWeights              []int64         `json:"rank_weights"`
	MinPayoutAmountCents     int64           `json:"min_payout_amount_cents"`
	PayoutMethod             string          `json:"payout_method"`
	PayoutChannel            string          `json:"payout_channel"`
	ChangeReason             string          `json:"change_reason"`
	CreatedBy                *int64          `json:"created_by,omitempty"`
	CreatedAt                time.Time       `json:"created_at"`
}

type CampaignParticipant struct {
	CampaignID                       int64     `json:"campaign_id"`
	UserID                           int64     `json:"user_id"`
	MaskedEmail                      string    `json:"masked_email,omitempty"`
	Username                         string    `json:"username,omitempty"`
	InviteCodeSnapshot               string    `json:"invite_code_snapshot"`
	InviteLinkSnapshot               string    `json:"invite_link_snapshot"`
	ParticipantStatus                string    `json:"participant_status"`
	JoinedAt                         time.Time `json:"joined_at"`
	ValidInviteCount                 int       `json:"valid_invite_count"`
	PendingInviteCount               int       `json:"pending_invite_count"`
	InvalidInviteCount               int       `json:"invalid_invite_count"`
	InviteeRechargeAmountCents       int64     `json:"invitee_recharge_amount_cents"`
	EstimatedRankRewardCents         int64     `json:"estimated_rank_reward_cents"`
	EstimatedContributionRewardCents int64     `json:"estimated_contribution_reward_cents"`
	EstimatedTotalRewardCents        int64     `json:"estimated_total_reward_cents"`
	FinalRankRewardCents             int64     `json:"final_rank_reward_cents"`
	FinalContributionRewardCents     int64     `json:"final_contribution_reward_cents"`
	FinalTotalRewardCents            int64     `json:"final_total_reward_cents"`
}

type CampaignInviteRecord struct {
	ID                           int64      `json:"id"`
	CampaignID                   int64      `json:"campaign_id"`
	ConfigVersionID              int64      `json:"config_version_id"`
	InviterUserID                int64      `json:"inviter_user_id"`
	InviteeUserID                int64      `json:"invitee_user_id"`
	InviteeMaskedEmail           string     `json:"invitee_masked_email,omitempty"`
	InviteeUsername              string     `json:"invitee_username,omitempty"`
	InviteSource                 string     `json:"invite_source"`
	ThresholdSnapshotCents       int64      `json:"threshold_snapshot_cents"`
	RegisteredAt                 time.Time  `json:"registered_at"`
	QualifiedAt                  *time.Time `json:"qualified_at,omitempty"`
	EffectiveRechargeAmountCents int64      `json:"effective_recharge_amount_cents"`
	Status                       string     `json:"status"`
	RiskLevel                    string     `json:"risk_level"`
	InvalidReason                string     `json:"invalid_reason"`
	AuditStatus                  string     `json:"audit_status"`
	AuditBy                      *int64     `json:"audit_by,omitempty"`
	AuditAt                      *time.Time `json:"audit_at,omitempty"`
	AuditNote                    string     `json:"audit_note"`
}

type CampaignManualLeaderboardAdjustment struct {
	ID                       int64     `json:"id"`
	CampaignID               int64     `json:"campaign_id"`
	UserID                   int64     `json:"user_id"`
	AdjustmentType           string    `json:"adjustment_type"`
	ValidInviteDelta         int       `json:"valid_invite_delta"`
	RechargeAmountDeltaCents int64     `json:"recharge_amount_delta_cents"`
	Reason                   string    `json:"reason"`
	OperatorID               *int64    `json:"operator_id,omitempty"`
	CreatedAt                time.Time `json:"created_at"`
}

type CampaignPoolSummary struct {
	CampaignID              int64 `json:"campaign_id"`
	ConfirmedPoolCents      int64 `json:"confirmed_pool_cents"`
	PendingPoolCents        int64 `json:"pending_pool_cents"`
	EstimatedTotalPoolCents int64 `json:"estimated_total_pool_cents"`
	AdjustmentTotalCents    int64 `json:"adjustment_total_cents"`
	DeductedPoolCents       int64 `json:"deducted_pool_cents"`
	FinalPoolCents          int64 `json:"final_pool_cents"`
}

type CampaignDeleteImpact struct {
	Participants         int64 `json:"participants"`
	InviteRecords        int64 `json:"invite_records"`
	HistoricalInvites    int64 `json:"historical_invites"`
	PoolEntries          int64 `json:"pool_entries"`
	PoolAdjustments      int64 `json:"pool_adjustments"`
	LeaderboardSnapshots int64 `json:"leaderboard_snapshots"`
	RewardResults        int64 `json:"reward_results"`
	PayoutBatches        int64 `json:"payout_batches"`
	PayoutItems          int64 `json:"payout_items"`
}

func (i CampaignDeleteImpact) HasBusinessData() bool {
	return i.Participants+i.InviteRecords+i.HistoricalInvites+i.PoolEntries+i.PoolAdjustments+
		i.LeaderboardSnapshots+i.RewardResults+i.PayoutBatches+i.PayoutItems > 0
}

type CampaignDeleteResult struct {
	Action   string               `json:"action"`
	Campaign *Campaign            `json:"campaign,omitempty"`
	Impact   CampaignDeleteImpact `json:"impact"`
}

type CampaignLeaderboardRow struct {
	Rank                       int       `json:"rank"`
	UserID                     int64     `json:"user_id"`
	MaskedEmail                string    `json:"masked_email,omitempty"`
	Username                   string    `json:"username,omitempty"`
	ValidInviteCount           float64   `json:"valid_invite_count"`
	ActivityValidInviteCount   int       `json:"activity_valid_invite_count"`
	HistoricalValidInviteCount int       `json:"historical_valid_invite_count"`
	HistoricalWeightedCount    float64   `json:"historical_weighted_invite_count"`
	PendingInviteCount         int       `json:"pending_invite_count"`
	InviteeRechargeAmountCents int64     `json:"invitee_recharge_amount_cents"`
	ManualValidInviteDelta     int       `json:"manual_valid_invite_delta"`
	ManualRechargeAmountCents  int64     `json:"manual_recharge_amount_delta_cents"`
	HasManualAdjustment        bool      `json:"has_manual_adjustment"`
	ReachedCountAt             time.Time `json:"reached_count_at"`
	JoinedAt                   time.Time `json:"joined_at"`
	EstimatedRewardCents       int64     `json:"estimated_reward_cents"`
	FinalRewardCents           int64     `json:"final_reward_cents"`
}

type CampaignRewardResult struct {
	ID                            int64           `json:"id"`
	CampaignID                    int64           `json:"campaign_id"`
	ConfigVersionID               int64           `json:"config_version_id"`
	CalculationBatchNo            string          `json:"calculation_batch_no"`
	UserID                        int64           `json:"user_id"`
	Rank                          *int            `json:"rank,omitempty"`
	RankRewardAmountCents         int64           `json:"rank_reward_amount_cents"`
	ContributionWeight            decimal.Decimal `json:"contribution_weight"`
	ContributionRewardAmountCents int64           `json:"contribution_reward_amount_cents"`
	GrossRewardAmountCents        int64           `json:"gross_reward_amount_cents"`
	MinPayoutAmountSnapshotCents  int64           `json:"min_payout_amount_snapshot_cents"`
	FinalPayoutAmountCents        int64           `json:"final_payout_amount_cents"`
	WithheldAmountCents           int64           `json:"withheld_amount_cents"`
	WithheldReason                string          `json:"withheld_reason"`
	RoundingResidualCents         int64           `json:"rounding_residual_cents"`
	CalculationStatus             string          `json:"calculation_status"`
	CalculatedAt                  time.Time       `json:"calculated_at"`
}

type CampaignPayoutBatch struct {
	ID               int64      `json:"id"`
	CampaignID       int64      `json:"campaign_id"`
	BatchNo          string     `json:"batch_no"`
	Status           string     `json:"status"`
	OperatorID       *int64     `json:"operator_id,omitempty"`
	TotalUsers       int        `json:"total_users"`
	TotalAmountCents int64      `json:"total_amount_cents"`
	SuccessCount     int        `json:"success_count"`
	FailedCount      int        `json:"failed_count"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type CampaignPayoutItem struct {
	ID                    int64      `json:"id"`
	BatchID               int64      `json:"batch_id"`
	CampaignID            int64      `json:"campaign_id"`
	UserID                int64      `json:"user_id"`
	RewardResultID        int64      `json:"reward_result_id"`
	AmountCents           int64      `json:"amount_cents"`
	BalanceBeforeSnapshot *float64   `json:"balance_before_snapshot,omitempty"`
	BalanceAfterSnapshot  *float64   `json:"balance_after_snapshot,omitempty"`
	Status                string     `json:"status"`
	IdempotencyKey        string     `json:"idempotency_key"`
	ErrorMessage          string     `json:"error_message"`
	ProcessedAt           *time.Time `json:"processed_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}

type CampaignCreateInput struct {
	Name                     string
	Description              string
	CoverURL                 string
	RulesText                string
	WarmupStartAt            *time.Time
	StartAt                  time.Time
	EndAt                    time.Time
	AuditStartAt             *time.Time
	AuditEndAt               *time.Time
	PublicityStartAt         *time.Time
	PublicityEndAt           *time.Time
	PayoutDueAt              *time.Time
	InitialBonusCents        int64
	RechargeThresholdCents   int64
	AllowAccumulatedRecharge bool
	HistoricalInviteRatio    decimal.Decimal
	PoolInjectionRate        decimal.Decimal
	PoolInjectionScope       string
	RankPoolRatio            decimal.Decimal
	ContributionPoolRatio    decimal.Decimal
	RankRewardCount          int
	RankWeights              []int64
	MinPayoutAmountCents     int64
	OperatorID               *int64
}

type CampaignUpdateInput struct {
	Name                  *string
	Description           *string
	CoverURL              *string
	RulesText             *string
	WarmupStartAt         **time.Time
	StartAt               *time.Time
	EndAt                 *time.Time
	AuditStartAt          **time.Time
	AuditEndAt            **time.Time
	PublicityStartAt      **time.Time
	PublicityEndAt        **time.Time
	PayoutDueAt           **time.Time
	HistoricalInviteRatio *decimal.Decimal
	OperatorID            *int64
}

type CampaignConfigVersionInput struct {
	VersionScope             string
	EffectiveAt              time.Time
	RechargeThresholdCents   *int64
	PoolInjectionRate        *decimal.Decimal
	PoolInjectionScope       *string
	AllowAccumulatedRecharge *bool
	ChangeReason             string
	OperatorID               *int64
}

type CampaignPoolAdjustmentInput struct {
	AdjustmentType string
	AmountCents    int64
	Reason         string
	OperatorID     *int64
}

type CampaignInviteRecordAdjustmentInput struct {
	RecordID                     int64
	Status                       string
	EffectiveRechargeAmountCents int64
	Reason                       string
	OperatorID                   *int64
}

type CampaignLeaderboardAdjustmentInput struct {
	UserID                   int64
	ValidInviteDelta         int
	RechargeAmountDeltaCents int64
	Reason                   string
	OperatorID               *int64
}

type CampaignRegisterInviteInput struct {
	InviteeUserID int64
	AffiliateCode string
	RegisteredAt  time.Time
	InviteSource  string
}

type CampaignRechargeInput struct {
	InviteeUserID       int64
	SourceType          string
	SourceID            string
	SourceSuccessAt     time.Time
	RechargeAmountCents int64
}

type CampaignDeductionInput struct {
	CampaignID        int64
	PoolEntryID       int64
	SourceType        string
	SourceID          string
	DeductAmountCents int64
	IdempotencyKey    string
	Reason            string
}

type CampaignCalculationSummary struct {
	CampaignID                 int64                  `json:"campaign_id"`
	CalculationStatus          string                 `json:"calculation_status"`
	CalculationBatchNo         string                 `json:"calculation_batch_no"`
	FinalPoolCents             int64                  `json:"final_pool_cents"`
	RankPoolCents              int64                  `json:"rank_pool_cents"`
	ContributionPoolCents      int64                  `json:"contribution_pool_cents"`
	TotalGrossRewardCents      int64                  `json:"total_gross_reward_cents"`
	TotalFinalPayoutCents      int64                  `json:"total_final_payout_cents"`
	TotalWithheldCents         int64                  `json:"total_withheld_cents"`
	TotalRoundingResidualCents int64                  `json:"total_rounding_residual_cents"`
	Results                    []CampaignRewardResult `json:"results"`
}

type CampaignHome struct {
	Campaign        *Campaign                `json:"campaign"`
	Config          *CampaignConfigVersion   `json:"config"`
	Pool            *CampaignPoolSummary     `json:"pool"`
	Leaderboard     []CampaignLeaderboardRow `json:"leaderboard"`
	DataDelayNotice string                   `json:"data_delay_notice"`
	EstimateNotice  string                   `json:"estimate_notice"`
}

type CampaignMyData struct {
	CampaignID                       int64                  `json:"campaign_id"`
	UserID                           int64                  `json:"user_id"`
	InviteCode                       string                 `json:"invite_code"`
	InviteLink                       string                 `json:"invite_link"`
	ValidInviteCount                 float64                `json:"valid_invite_count"`
	ActivityValidInviteCount         int                    `json:"activity_valid_invite_count"`
	HistoricalValidInviteCount       int                    `json:"historical_valid_invite_count"`
	HistoricalWeightedInviteCount    float64                `json:"historical_weighted_invite_count"`
	PendingInviteCount               int                    `json:"pending_invite_count"`
	InvalidInviteCount               int                    `json:"invalid_invite_count"`
	CurrentRank                      *int                   `json:"current_rank,omitempty"`
	EstimatedRankRewardCents         int64                  `json:"estimated_rank_reward_cents"`
	EstimatedContributionRewardCents int64                  `json:"estimated_contribution_reward_cents"`
	EstimatedTotalRewardCents        int64                  `json:"estimated_total_reward_cents"`
	InviteeRechargeAmountCents       int64                  `json:"invitee_recharge_amount_cents"`
	DistanceToPrevious               float64                `json:"distance_to_previous"`
	DistanceToTop10                  float64                `json:"distance_to_top10"`
	InviteRecords                    []CampaignInviteRecord `json:"invite_records"`
}

type CampaignRepository interface {
	CreateCampaign(ctx context.Context, input CampaignCreateInput, cfg CampaignConfigVersion) (*Campaign, *CampaignConfigVersion, error)
	UpdateCampaign(ctx context.Context, campaignID int64, input CampaignUpdateInput) (*Campaign, error)
	GetCampaign(ctx context.Context, campaignID int64) (*Campaign, error)
	ListCampaigns(ctx context.Context, page, pageSize int) ([]Campaign, int64, error)
	GetActiveCampaign(ctx context.Context, now time.Time) (*Campaign, error)
	CreateConfigVersion(ctx context.Context, campaignID int64, input CampaignConfigVersionInput, cfg CampaignConfigVersion) (*CampaignConfigVersion, error)
	GetLatestConfigVersion(ctx context.Context, campaignID int64) (*CampaignConfigVersion, error)
	GetLatestConfigVersionAt(ctx context.Context, campaignID int64, at time.Time) (*CampaignConfigVersion, error)
	GetPublishedConfigVersion(ctx context.Context, campaignID int64) (*CampaignConfigVersion, error)
	PublishCampaign(ctx context.Context, campaignID int64, operatorID *int64, now time.Time) (*Campaign, error)
	UpdateCampaignStatus(ctx context.Context, campaignID int64, status string, operatorID *int64) (*Campaign, error)
	HasActiveCampaign(ctx context.Context, excludeCampaignID int64) (bool, error)
	GetCampaignDeleteImpact(ctx context.Context, campaignID int64) (*CampaignDeleteImpact, error)
	DeleteCampaign(ctx context.Context, campaignID int64) error

	GetInviterByAffiliateCode(ctx context.Context, code string) (*AffiliateSummary, error)
	RecordInviteRegistration(ctx context.Context, campaign *Campaign, cfg *CampaignConfigVersion, inviter *AffiliateSummary, input CampaignRegisterInviteInput) (*CampaignInviteRecord, error)
	RecordRecharge(ctx context.Context, campaign *Campaign, cfg *CampaignConfigVersion, input CampaignRechargeInput, poolAmountCents int64) (*CampaignInviteRecord, bool, error)
	InsertPoolEntry(ctx context.Context, campaign *Campaign, cfg *CampaignConfigVersion, invite *CampaignInviteRecord, input CampaignRechargeInput, poolAmountCents int64) (bool, error)
	ListInviteRecords(ctx context.Context, campaignID, inviterUserID int64, page, pageSize int) ([]CampaignInviteRecord, int64, error)
	AdjustInviteRecord(ctx context.Context, campaignID int64, input CampaignInviteRecordAdjustmentInput) (*CampaignInviteRecord, error)
	AddLeaderboardAdjustment(ctx context.Context, campaignID int64, input CampaignLeaderboardAdjustmentInput) (*CampaignManualLeaderboardAdjustment, error)
	GetParticipantStats(ctx context.Context, campaignID, userID int64) (*CampaignParticipant, error)
	ListLeaderboardRows(ctx context.Context, campaignID int64, limit int) ([]CampaignLeaderboardRow, error)
	GetLeaderboardRowForUser(ctx context.Context, campaignID, userID int64) (*CampaignLeaderboardRow, error)
	ListRewardEligibleRows(ctx context.Context, campaignID int64) ([]CampaignLeaderboardRow, error)

	AddPoolAdjustment(ctx context.Context, campaignID int64, input CampaignPoolAdjustmentInput) error
	GetPoolSummary(ctx context.Context, campaignID int64) (*CampaignPoolSummary, error)
	InsertPoolDeduction(ctx context.Context, input CampaignDeductionInput) (bool, error)

	SaveLeaderboardSnapshot(ctx context.Context, campaignID int64, snapshotType string, rows []CampaignLeaderboardRow) error
	SaveRewardResults(ctx context.Context, campaignID int64, status string, batchNo string, results []CampaignRewardResult) error
	ListRewardResults(ctx context.Context, campaignID int64, status string) ([]CampaignRewardResult, error)

	CreatePayoutBatch(ctx context.Context, campaignID int64, batchNo string, operatorID *int64, results []CampaignRewardResult) (*CampaignPayoutBatch, error)
	GetPayoutBatchForRewardResult(ctx context.Context, campaignID, rewardResultID int64) (*CampaignPayoutBatch, error)
	MarkPayoutItemSuccess(ctx context.Context, itemID int64, before, after float64) error
	MarkPayoutItemFailed(ctx context.Context, itemID int64, errMessage string) error
	ListPayoutItems(ctx context.Context, batchID int64) ([]CampaignPayoutItem, error)
	UpdatePayoutBatchSummary(ctx context.Context, batchID int64) (*CampaignPayoutBatch, error)
	GetSuccessfulPayoutBatch(ctx context.Context, campaignID int64) (*CampaignPayoutBatch, error)
}

type campaignAffiliateProfileRepository interface {
	EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error)
}

type campaignHistoricalSnapshotRepository interface {
	EnsureHistoricalInviteSnapshot(ctx context.Context, campaign *Campaign, cfg *CampaignConfigVersion) error
}

type CampaignBalanceGrantService interface {
	GrantUserBalances(ctx context.Context, grants []BalanceGrantInput, notes string) ([]BalanceGrantResult, error)
}

type CampaignService struct {
	repo         CampaignRepository
	balanceGrant CampaignBalanceGrantService
}

func NewCampaignService(repo CampaignRepository, adminService AdminService) *CampaignService {
	return &CampaignService{repo: repo, balanceGrant: adminService}
}

func (s *CampaignService) CreateCampaign(ctx context.Context, input CampaignCreateInput) (*Campaign, *CampaignConfigVersion, error) {
	if s == nil || s.repo == nil {
		return nil, nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "campaign service unavailable")
	}
	cfg, err := buildInitialCampaignConfig(input)
	if err != nil {
		return nil, nil, err
	}
	if err := validateCampaignTime(input.StartAt, input.EndAt); err != nil {
		return nil, nil, err
	}
	if input.InitialBonusCents < 0 {
		return nil, nil, ErrCampaignInvalidConfig
	}
	campaign, version, err := s.repo.CreateCampaign(ctx, input, cfg)
	if err != nil {
		return nil, nil, err
	}
	if input.InitialBonusCents > 0 {
		_ = s.repo.AddPoolAdjustment(ctx, campaign.ID, CampaignPoolAdjustmentInput{
			AdjustmentType: "initial_bonus",
			AmountCents:    input.InitialBonusCents,
			Reason:         "活动初始奖金池",
			OperatorID:     input.OperatorID,
		})
	}
	return campaign, version, nil
}

func (s *CampaignService) CopyCampaign(ctx context.Context, campaignID int64, operatorID *int64) (*Campaign, *CampaignConfigVersion, error) {
	if s == nil || s.repo == nil {
		return nil, nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "campaign service unavailable")
	}
	if campaignID <= 0 {
		return nil, nil, ErrCampaignNotFound
	}
	source, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := s.repo.GetLatestConfigVersion(ctx, campaignID)
	if err != nil {
		return nil, nil, err
	}
	input := CampaignCreateInput{
		Name:                     copiedCampaignName(source.Name),
		Description:              source.Description,
		CoverURL:                 source.CoverURL,
		RulesText:                source.RulesText,
		WarmupStartAt:            source.WarmupStartAt,
		StartAt:                  source.StartAt,
		EndAt:                    source.EndAt,
		AuditStartAt:             source.AuditStartAt,
		AuditEndAt:               source.AuditEndAt,
		PublicityStartAt:         source.PublicityStartAt,
		PublicityEndAt:           source.PublicityEndAt,
		PayoutDueAt:              source.PayoutDueAt,
		InitialBonusCents:        0,
		RechargeThresholdCents:   cfg.RechargeThresholdCents,
		AllowAccumulatedRecharge: cfg.AllowAccumulatedRecharge,
		HistoricalInviteRatio:    cfg.HistoricalInviteRatio,
		PoolInjectionRate:        cfg.PoolInjectionRate,
		PoolInjectionScope:       normalizedCampaignPoolInjectionScope(cfg.PoolInjectionScope),
		RankPoolRatio:            cfg.RankPoolRatio,
		ContributionPoolRatio:    cfg.ContributionPoolRatio,
		RankRewardCount:          cfg.RankRewardCount,
		RankWeights:              append([]int64(nil), cfg.RankWeights...),
		MinPayoutAmountCents:     cfg.MinPayoutAmountCents,
		OperatorID:               operatorID,
	}
	return s.CreateCampaign(ctx, input)
}

func (s *CampaignService) UpdateCampaign(ctx context.Context, campaignID int64, input CampaignUpdateInput) (*Campaign, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "campaign service unavailable")
	}
	if campaignID <= 0 {
		return nil, ErrCampaignNotFound
	}
	current, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	nextStart := current.StartAt
	if input.StartAt != nil {
		nextStart = *input.StartAt
	}
	nextEnd := current.EndAt
	if input.EndAt != nil {
		nextEnd = *input.EndAt
	}
	if err := validateCampaignTime(nextStart, nextEnd); err != nil {
		return nil, err
	}
	if input.HistoricalInviteRatio != nil {
		if input.HistoricalInviteRatio.IsNegative() || input.HistoricalInviteRatio.GreaterThan(decimal.NewFromInt(1)) {
			return nil, ErrCampaignInvalidConfig
		}
	}
	return s.repo.UpdateCampaign(ctx, campaignID, input)
}

func (s *CampaignService) ListCampaigns(ctx context.Context, page, pageSize int) ([]Campaign, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.ListCampaigns(ctx, page, pageSize)
}

func (s *CampaignService) GetCampaign(ctx context.Context, campaignID int64) (*Campaign, error) {
	if campaignID <= 0 {
		return nil, ErrCampaignNotFound
	}
	return s.repo.GetCampaign(ctx, campaignID)
}

func (s *CampaignService) GetCampaignConfig(ctx context.Context, campaignID int64) (*CampaignConfigVersion, error) {
	cfg, err := s.repo.GetPublishedConfigVersion(ctx, campaignID)
	if err == nil {
		return cfg, nil
	}
	return s.repo.GetLatestConfigVersion(ctx, campaignID)
}

func (s *CampaignService) DeleteCampaign(ctx context.Context, campaignID int64, operatorID *int64) (*CampaignDeleteResult, error) {
	if campaignID <= 0 {
		return nil, ErrCampaignNotFound
	}
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if campaign.Status == CampaignStatusPaid {
		return nil, ErrCampaignDeleteBlocked
	}
	impact, err := s.repo.GetCampaignDeleteImpact(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if !impact.HasBusinessData() && (campaign.Status == CampaignStatusDraft || campaign.Status == CampaignStatusWarmup) {
		if err := s.repo.DeleteCampaign(ctx, campaignID); err != nil {
			return nil, err
		}
		return &CampaignDeleteResult{Action: "deleted", Impact: *impact}, nil
	}
	if campaign.Status == CampaignStatusCancelled || campaign.Status == CampaignStatusTerminated {
		return &CampaignDeleteResult{Action: "archived", Campaign: campaign, Impact: *impact}, nil
	}
	nextStatus := CampaignStatusCancelled
	if campaign.Status == CampaignStatusActive || campaign.Status == CampaignStatusPaused || campaign.Status == CampaignStatusFrozen {
		nextStatus = CampaignStatusTerminated
	}
	archived, err := s.repo.UpdateCampaignStatus(ctx, campaignID, nextStatus, operatorID)
	if err != nil {
		return nil, err
	}
	return &CampaignDeleteResult{Action: "archived", Campaign: archived, Impact: *impact}, nil
}

func (s *CampaignService) PublishCampaign(ctx context.Context, campaignID int64, operatorID *int64) (*Campaign, error) {
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if campaign.Status != CampaignStatusDraft && campaign.Status != CampaignStatusWarmup {
		return nil, ErrCampaignImmutableRule
	}
	now := time.Now()
	if !now.Before(campaign.StartAt) && now.Before(campaign.EndAt) {
		hasActive, err := s.repo.HasActiveCampaign(ctx, campaignID)
		if err != nil {
			return nil, err
		}
		if hasActive {
			return nil, ErrCampaignDuplicateActive
		}
	}
	return s.repo.PublishCampaign(ctx, campaignID, operatorID, now)
}

func (s *CampaignService) CreateConfigVersion(ctx context.Context, campaignID int64, input CampaignConfigVersionInput) (*CampaignConfigVersion, error) {
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if campaign.Status == CampaignStatusPaid || campaign.Status == CampaignStatusCancelled || campaign.Status == CampaignStatusTerminated {
		return nil, ErrCampaignImmutableRule
	}
	if input.VersionScope != CampaignConfigScopeThresholdAdjustment && input.VersionScope != CampaignConfigScopeInjectionRateAdjust && input.VersionScope != CampaignConfigScopePoolScopeAdjustment {
		return nil, ErrCampaignImmutableRule
	}
	base, err := s.repo.GetLatestConfigVersionAt(ctx, campaignID, input.EffectiveAt)
	if err != nil {
		return nil, err
	}
	next := *base
	next.VersionScope = input.VersionScope
	next.EffectiveAt = input.EffectiveAt
	next.ChangeReason = strings.TrimSpace(input.ChangeReason)
	next.CreatedBy = input.OperatorID
	if input.RechargeThresholdCents != nil {
		next.RechargeThresholdCents = *input.RechargeThresholdCents
	}
	if input.PoolInjectionRate != nil {
		next.PoolInjectionRate = *input.PoolInjectionRate
	}
	if input.PoolInjectionScope != nil {
		next.PoolInjectionScope = normalizedCampaignPoolInjectionScope(*input.PoolInjectionScope)
	}
	if input.AllowAccumulatedRecharge != nil {
		next.AllowAccumulatedRecharge = *input.AllowAccumulatedRecharge
	}
	if err := validateCampaignConfig(next); err != nil {
		return nil, err
	}
	return s.repo.CreateConfigVersion(ctx, campaignID, input, next)
}

func (s *CampaignService) AddPoolAdjustment(ctx context.Context, campaignID int64, input CampaignPoolAdjustmentInput) error {
	if campaignID <= 0 || strings.TrimSpace(input.AdjustmentType) == "" {
		return ErrCampaignInvalidConfig
	}
	if input.AmountCents == 0 {
		return ErrCampaignInvalidConfig
	}
	if err := s.ensureCampaignAdjustable(ctx, campaignID); err != nil {
		return err
	}
	return s.repo.AddPoolAdjustment(ctx, campaignID, input)
}

func (s *CampaignService) PoolSummary(ctx context.Context, campaignID int64) (*CampaignPoolSummary, error) {
	if campaignID <= 0 {
		return nil, ErrCampaignNotFound
	}
	return s.repo.GetPoolSummary(ctx, campaignID)
}

func (s *CampaignService) ListInviteRecords(ctx context.Context, campaignID, inviterUserID int64, page, pageSize int) ([]CampaignInviteRecord, int64, error) {
	if campaignID <= 0 || inviterUserID <= 0 {
		return nil, 0, ErrCampaignNotFound
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.ListInviteRecords(ctx, campaignID, inviterUserID, page, pageSize)
}

func (s *CampaignService) AdjustInviteRecord(ctx context.Context, campaignID int64, input CampaignInviteRecordAdjustmentInput) (*CampaignInviteRecord, error) {
	if campaignID <= 0 || input.RecordID <= 0 {
		return nil, ErrCampaignNotFound
	}
	if !isCampaignInviteAdjustableStatus(input.Status) || input.EffectiveRechargeAmountCents < 0 {
		return nil, ErrCampaignInvalidConfig
	}
	if err := s.ensureCampaignAdjustable(ctx, campaignID); err != nil {
		return nil, err
	}
	input.Reason = strings.TrimSpace(input.Reason)
	return s.repo.AdjustInviteRecord(ctx, campaignID, input)
}

func (s *CampaignService) AddLeaderboardAdjustment(ctx context.Context, campaignID int64, input CampaignLeaderboardAdjustmentInput) (*CampaignManualLeaderboardAdjustment, error) {
	if campaignID <= 0 || input.UserID <= 0 {
		return nil, ErrCampaignNotFound
	}
	if input.ValidInviteDelta == 0 && input.RechargeAmountDeltaCents == 0 {
		return nil, ErrCampaignInvalidConfig
	}
	if err := s.ensureCampaignAdjustable(ctx, campaignID); err != nil {
		return nil, err
	}
	input.Reason = strings.TrimSpace(input.Reason)
	return s.repo.AddLeaderboardAdjustment(ctx, campaignID, input)
}

func (s *CampaignService) ensureCampaignAdjustable(ctx context.Context, campaignID int64) error {
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return err
	}
	if campaign.Status == CampaignStatusPaid {
		return ErrCampaignAlreadyPaid
	}
	if paid, err := s.repo.GetSuccessfulPayoutBatch(ctx, campaignID); err == nil && paid != nil {
		return ErrCampaignAlreadyPaid
	} else if err != nil && !errors.Is(err, ErrCampaignNotFound) {
		return err
	}
	return nil
}

func isCampaignInviteAdjustableStatus(status string) bool {
	switch status {
	case CampaignInviteStatusRegistered, CampaignInviteStatusRechargeUnqualified, CampaignInviteStatusPendingAudit, CampaignInviteStatusEffective, CampaignInviteStatusInvalid, CampaignInviteStatusRiskReview:
		return true
	default:
		return false
	}
}

func (s *CampaignService) GetActiveHome(ctx context.Context) (*CampaignHome, error) {
	now := time.Now()
	campaign, err := s.repo.GetActiveCampaign(ctx, now)
	if err != nil {
		if errors.Is(err, ErrCampaignNotFound) {
			return &CampaignHome{
				DataDelayNotice: "活动数据每 1 至 5 分钟更新一次。",
				EstimateNotice:  "当前没有进行中的邀请奖励活动。",
			}, nil
		}
		return nil, err
	}
	cfg, err := s.repo.GetLatestConfigVersionAt(ctx, campaign.ID, now)
	if err != nil {
		return nil, err
	}
	pool, err := s.repo.GetPoolSummary(ctx, campaign.ID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Leaderboard(ctx, campaign.ID, 50)
	if err != nil {
		return nil, err
	}
	return &CampaignHome{
		Campaign:        campaign,
		Config:          cfg,
		Pool:            pool,
		Leaderboard:     rows,
		DataDelayNotice: "数据每 1 至 5 分钟更新一次。",
		EstimateNotice:  "当前奖金池和奖励为预估金额，最终以活动结束后的审核结算为准。",
	}, nil
}

func (s *CampaignService) GetMyData(ctx context.Context, campaignID, userID int64) (*CampaignMyData, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Leaderboard(ctx, campaignID, 50)
	if err != nil {
		return nil, err
	}
	invites, _, err := s.repo.ListInviteRecords(ctx, campaignID, userID, 1, 100)
	if err != nil {
		return nil, err
	}
	participant, err := s.repo.GetParticipantStats(ctx, campaignID, userID)
	if err != nil && !errors.Is(err, ErrCampaignNotFound) {
		return nil, err
	}
	var result CampaignMyData
	result.CampaignID = campaign.ID
	result.UserID = userID
	if participant != nil {
		result.InviteCode = participant.InviteCodeSnapshot
		result.InviteLink = participant.InviteLinkSnapshot
		result.ValidInviteCount = float64(participant.ValidInviteCount)
		result.PendingInviteCount = participant.PendingInviteCount
		result.InvalidInviteCount = participant.InvalidInviteCount
		result.InviteeRechargeAmountCents = participant.InviteeRechargeAmountCents
		result.EstimatedRankRewardCents = participant.EstimatedRankRewardCents
		result.EstimatedContributionRewardCents = participant.EstimatedContributionRewardCents
		result.EstimatedTotalRewardCents = participant.EstimatedTotalRewardCents
	}
	if err := fillCampaignInviteIdentity(ctx, s.repo, &result); err != nil {
		return nil, err
	}
	var previousCount float64
	for i, row := range rows {
		if row.UserID == userID {
			rank := row.Rank
			result.CurrentRank = &rank
			result.ValidInviteCount = row.ValidInviteCount
			result.ActivityValidInviteCount = row.ActivityValidInviteCount
			result.HistoricalValidInviteCount = row.HistoricalValidInviteCount
			result.HistoricalWeightedInviteCount = row.HistoricalWeightedCount
			result.PendingInviteCount = row.PendingInviteCount
			result.InviteeRechargeAmountCents = row.InviteeRechargeAmountCents
			result.EstimatedTotalRewardCents = row.EstimatedRewardCents
			if i > 0 {
				previousCount = rows[i-1].ValidInviteCount
			}
			break
		}
	}
	if result.CurrentRank == nil {
		row, err := s.repo.GetLeaderboardRowForUser(ctx, campaignID, userID)
		if err != nil && !errors.Is(err, ErrCampaignNotFound) {
			return nil, err
		}
		if row != nil {
			rank := row.Rank
			result.CurrentRank = &rank
			result.ValidInviteCount = row.ValidInviteCount
			result.ActivityValidInviteCount = row.ActivityValidInviteCount
			result.HistoricalValidInviteCount = row.HistoricalValidInviteCount
			result.HistoricalWeightedInviteCount = row.HistoricalWeightedCount
			result.PendingInviteCount = row.PendingInviteCount
			result.InviteeRechargeAmountCents = row.InviteeRechargeAmountCents
			result.EstimatedTotalRewardCents = row.EstimatedRewardCents
		}
	}
	if result.ValidInviteCount > 0 && previousCount > result.ValidInviteCount {
		result.DistanceToPrevious = previousCount - result.ValidInviteCount
	}
	if len(rows) >= 10 {
		top10Count := rows[9].ValidInviteCount
		if result.ValidInviteCount < top10Count {
			result.DistanceToTop10 = top10Count - result.ValidInviteCount
		}
	}
	if participant == nil {
		for _, invite := range invites {
			switch invite.Status {
			case CampaignInviteStatusEffective:
				result.ValidInviteCount++
				result.InviteeRechargeAmountCents += invite.EffectiveRechargeAmountCents
			case CampaignInviteStatusPendingAudit, CampaignInviteStatusRiskReview, CampaignInviteStatusRechargeUnqualified, CampaignInviteStatusRegistered:
				result.PendingInviteCount++
			default:
				result.InvalidInviteCount++
			}
		}
	}
	result.InviteRecords = invites
	return &result, nil
}

func fillCampaignInviteIdentity(ctx context.Context, repo CampaignRepository, result *CampaignMyData) error {
	if result == nil {
		return nil
	}
	code := strings.TrimSpace(result.InviteCode)
	if code == "" {
		profileRepo, ok := repo.(campaignAffiliateProfileRepository)
		if !ok {
			return nil
		}
		summary, err := profileRepo.EnsureUserAffiliate(ctx, result.UserID)
		if err != nil {
			if errors.Is(err, ErrAffiliateProfileNotFound) || errors.Is(err, ErrCampaignNotFound) {
				return nil
			}
			return err
		}
		if summary != nil {
			code = strings.TrimSpace(summary.AffCode)
		}
	}
	if strings.TrimSpace(result.InviteCode) == "" {
		result.InviteCode = code
	}
	if strings.TrimSpace(result.InviteLink) == "" && code != "" {
		result.InviteLink = campaignInviteLink(code)
	}
	return nil
}

func campaignInviteLink(inviteCode string) string {
	code := strings.TrimSpace(inviteCode)
	if code == "" {
		return ""
	}
	return "/register?aff=" + url.QueryEscape(code)
}

func (s *CampaignService) RegisterInvite(ctx context.Context, input CampaignRegisterInviteInput) (*CampaignInviteRecord, error) {
	if s == nil || s.repo == nil || input.InviteeUserID <= 0 || strings.TrimSpace(input.AffiliateCode) == "" {
		return nil, nil
	}
	registeredAt := input.RegisteredAt
	if registeredAt.IsZero() {
		registeredAt = time.Now()
		input.RegisteredAt = registeredAt
	}
	campaign, err := s.repo.GetActiveCampaign(ctx, registeredAt)
	if err != nil {
		if errors.Is(err, ErrCampaignNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if registeredAt.Before(campaign.StartAt) || !registeredAt.Before(campaign.EndAt) {
		return nil, nil
	}
	cfg, err := s.repo.GetLatestConfigVersionAt(ctx, campaign.ID, registeredAt)
	if err != nil {
		return nil, err
	}
	inviter, err := s.repo.GetInviterByAffiliateCode(ctx, input.AffiliateCode)
	if err != nil {
		return nil, nil
	}
	if inviter == nil || inviter.UserID <= 0 || inviter.UserID == input.InviteeUserID {
		return nil, nil
	}
	if strings.TrimSpace(input.InviteSource) == "" {
		input.InviteSource = "affiliate_code"
	}
	return s.repo.RecordInviteRegistration(ctx, campaign, cfg, inviter, input)
}

func (s *CampaignService) RecordRecharge(ctx context.Context, input CampaignRechargeInput) (*CampaignInviteRecord, error) {
	if s == nil || s.repo == nil || input.InviteeUserID <= 0 || input.RechargeAmountCents <= 0 {
		return nil, nil
	}
	if input.SourceSuccessAt.IsZero() {
		input.SourceSuccessAt = time.Now()
	}
	campaign, err := s.repo.GetActiveCampaign(ctx, input.SourceSuccessAt)
	if err != nil {
		if errors.Is(err, ErrCampaignNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if input.SourceSuccessAt.Before(campaign.StartAt) || !input.SourceSuccessAt.Before(campaign.EndAt) {
		return nil, nil
	}
	cfg, err := s.repo.GetLatestConfigVersionAt(ctx, campaign.ID, input.SourceSuccessAt)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.SourceType) == "" || strings.TrimSpace(input.SourceID) == "" {
		return nil, ErrCampaignInvalidConfig
	}
	poolAmount := calculatePoolInjectionCents(input.RechargeAmountCents, cfg.PoolInjectionRate)
	invite, _, err := s.repo.RecordRecharge(ctx, campaign, cfg, input, poolAmount)
	if errors.Is(err, ErrCampaignNotFound) {
		if normalizedCampaignPoolInjectionScope(cfg.PoolInjectionScope) != CampaignPoolInjectionScopeAllUsers {
			return nil, nil
		}
		_, err = s.repo.InsertPoolEntry(ctx, campaign, cfg, nil, input, poolAmount)
		return nil, err
	}
	return invite, err
}

func (s *CampaignService) ConfirmPoolDeduction(ctx context.Context, input CampaignDeductionInput) (bool, error) {
	if input.CampaignID <= 0 || input.PoolEntryID <= 0 || input.DeductAmountCents <= 0 || strings.TrimSpace(input.IdempotencyKey) == "" {
		return false, ErrCampaignInvalidConfig
	}
	return s.repo.InsertPoolDeduction(ctx, input)
}

func (s *CampaignService) Leaderboard(ctx context.Context, campaignID int64, limit int) ([]CampaignLeaderboardRow, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	cfg, configErr := s.repo.GetPublishedConfigVersion(ctx, campaignID)
	if configErr == nil {
		if snapshotter, ok := s.repo.(campaignHistoricalSnapshotRepository); ok {
			if err := snapshotter.EnsureHistoricalInviteSnapshot(ctx, campaign, cfg); err != nil {
				return nil, err
			}
		}
	}
	rows, err := s.repo.ListLeaderboardRows(ctx, campaignID, limit)
	if err != nil {
		return nil, err
	}
	pool, err := s.repo.GetPoolSummary(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if configErr != nil {
		cfg, err = s.repo.GetLatestConfigVersionAt(ctx, campaignID, time.Now())
		if err != nil {
			return nil, err
		}
	}
	estimated := calculateCampaignRewards(campaignID, cfg, pool.FinalPoolCents, rows, CampaignCalculationPreview, "preview")
	byUser := make(map[int64]int64, len(estimated.Results))
	for _, result := range estimated.Results {
		byUser[result.UserID] = result.FinalPayoutAmountCents
	}
	for i := range rows {
		rows[i].EstimatedRewardCents = byUser[rows[i].UserID]
	}
	return rows, nil
}

func (s *CampaignService) FreezeLeaderboard(ctx context.Context, campaignID int64, operatorID *int64) error {
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return err
	}
	if campaign.Status == CampaignStatusFrozen {
		return nil
	}
	if campaign.Status != CampaignStatusActive {
		return ErrCampaignImmutableRule
	}
	rows, err := s.Leaderboard(ctx, campaignID, 200)
	if err != nil {
		return err
	}
	if err := s.repo.SaveLeaderboardSnapshot(ctx, campaignID, "end_frozen", rows); err != nil {
		return err
	}
	_, err = s.repo.UpdateCampaignStatus(ctx, campaignID, CampaignStatusFrozen, operatorID)
	return err
}

func (s *CampaignService) PauseCampaign(ctx context.Context, campaignID int64, operatorID *int64) (*Campaign, error) {
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if campaign.Status == CampaignStatusPaused {
		return campaign, nil
	}
	if campaign.Status != CampaignStatusWarmup && campaign.Status != CampaignStatusActive {
		return nil, ErrCampaignImmutableRule
	}
	return s.repo.UpdateCampaignStatus(ctx, campaignID, CampaignStatusPaused, operatorID)
}

func (s *CampaignService) resolveResumedCampaignStatus(ctx context.Context, campaignID int64, campaign *Campaign) (string, error) {
	now := time.Now()
	nextStatus := CampaignStatusAuditing
	if now.Before(campaign.StartAt) {
		nextStatus = CampaignStatusWarmup
	} else if now.Before(campaign.EndAt) {
		hasActive, err := s.repo.HasActiveCampaign(ctx, campaignID)
		if err != nil {
			return "", err
		}
		if hasActive {
			return "", ErrCampaignDuplicateActive
		}
		nextStatus = CampaignStatusActive
	}
	return nextStatus, nil
}

func (s *CampaignService) ResumeCampaign(ctx context.Context, campaignID int64, operatorID *int64) (*Campaign, error) {
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if campaign.Status != CampaignStatusPaused {
		return nil, ErrCampaignImmutableRule
	}
	nextStatus, err := s.resolveResumedCampaignStatus(ctx, campaignID, campaign)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateCampaignStatus(ctx, campaignID, nextStatus, operatorID)
}

func (s *CampaignService) UnfreezeLeaderboard(ctx context.Context, campaignID int64, operatorID *int64) (*Campaign, error) {
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if campaign.Status != CampaignStatusFrozen {
		return nil, ErrCampaignImmutableRule
	}
	results, err := s.repo.ListRewardResults(ctx, campaignID, CampaignCalculationFinal)
	if err != nil {
		return nil, err
	}
	if len(results) > 0 {
		return nil, ErrCampaignSettlementLocked
	}
	if payout, err := s.repo.GetSuccessfulPayoutBatch(ctx, campaignID); err == nil && payout != nil {
		return nil, ErrCampaignSettlementLocked
	} else if err != nil && !errors.Is(err, ErrCampaignNotFound) {
		return nil, err
	}
	nextStatus, err := s.resolveResumedCampaignStatus(ctx, campaignID, campaign)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateCampaignStatus(ctx, campaignID, nextStatus, operatorID)
}

func (s *CampaignService) RecalculateRewards(ctx context.Context, campaignID int64, status string) (*CampaignCalculationSummary, error) {
	if status == "" {
		status = CampaignCalculationPreview
	}
	campaign, err := s.repo.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if status == CampaignCalculationFinal && time.Now().Before(campaign.EndAt) {
		return nil, ErrCampaignInvalidConfig
	}
	cfg, err := s.repo.GetPublishedConfigVersion(ctx, campaignID)
	if err != nil {
		if campaign.Status == CampaignStatusDraft {
			cfg, err = s.repo.GetLatestConfigVersionAt(ctx, campaignID, time.Now())
		}
		if err != nil {
			return nil, err
		}
	}
	pool, err := s.repo.GetPoolSummary(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListRewardEligibleRows(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	batchNo := fmt.Sprintf("%s-%d", status, time.Now().UnixNano())
	summary := calculateCampaignRewards(campaignID, cfg, pool.FinalPoolCents, rows, status, batchNo)
	if err := s.repo.SaveRewardResults(ctx, campaignID, status, batchNo, summary.Results); err != nil {
		return nil, err
	}
	if status == CampaignCalculationFinal {
		finalRows := make([]CampaignLeaderboardRow, 0, len(rows))
		resultByUser := make(map[int64]CampaignRewardResult, len(summary.Results))
		for _, result := range summary.Results {
			resultByUser[result.UserID] = result
		}
		for _, row := range rows {
			if result, ok := resultByUser[row.UserID]; ok {
				row.FinalRewardCents = result.FinalPayoutAmountCents
			}
			finalRows = append(finalRows, row)
		}
		_ = s.repo.SaveLeaderboardSnapshot(ctx, campaignID, "final", finalRows)
	}
	return summary, nil
}

func (s *CampaignService) GetFinalRewardResults(ctx context.Context, campaignID int64) (*CampaignCalculationSummary, error) {
	if campaignID <= 0 {
		return nil, ErrCampaignNotFound
	}
	pool, err := s.repo.GetPoolSummary(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	cfg, err := s.repo.GetPublishedConfigVersion(ctx, campaignID)
	if err != nil {
		cfg, err = s.repo.GetLatestConfigVersionAt(ctx, campaignID, time.Now())
		if err != nil {
			return nil, err
		}
	}
	results, err := s.repo.ListRewardResults(ctx, campaignID, CampaignCalculationFinal)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, ErrCampaignNoFinalSettlement
	}
	return summarizeCampaignRewardResults(campaignID, cfg, pool.FinalPoolCents, CampaignCalculationFinal, results), nil
}

func (s *CampaignService) Payout(ctx context.Context, campaignID int64, operatorID *int64) (*CampaignPayoutBatch, error) {
	if s.balanceGrant == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "balance grant service unavailable")
	}
	var retryBatch *CampaignPayoutBatch
	if paid, err := s.repo.GetSuccessfulPayoutBatch(ctx, campaignID); err == nil && paid != nil {
		switch paid.Status {
		case "success", "processing":
			return nil, ErrCampaignAlreadyPaid
		case "partial_success", "failed":
			retryBatch = paid
		}
	}
	results, err := s.repo.ListRewardResults(ctx, campaignID, CampaignCalculationFinal)
	if err != nil {
		return nil, err
	}
	payable := make([]CampaignRewardResult, 0, len(results))
	for _, result := range results {
		if result.FinalPayoutAmountCents > 0 {
			existing, err := s.repo.GetPayoutBatchForRewardResult(ctx, campaignID, result.ID)
			if err != nil && !errors.Is(err, ErrCampaignNotFound) {
				return nil, err
			}
			if existing != nil && (retryBatch == nil || existing.ID != retryBatch.ID) {
				return nil, ErrCampaignAlreadyPaid
			}
			payable = append(payable, result)
		}
	}
	if len(payable) == 0 {
		return nil, ErrCampaignNoFinalSettlement
	}
	batch := retryBatch
	if batch == nil {
		batchNo := fmt.Sprintf("campaign-%d-%d", campaignID, time.Now().UnixNano())
		var err error
		batch, err = s.repo.CreatePayoutBatch(ctx, campaignID, batchNo, operatorID, payable)
		if err != nil {
			return nil, err
		}
	}
	items, err := s.repo.ListPayoutItems(ctx, batch.ID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Status == "success" {
			continue
		}
		amount := centsToYuanFloat(item.AmountCents)
		results, grantErr := s.balanceGrant.GrantUserBalances(ctx, []BalanceGrantInput{{UserID: item.UserID, Amount: amount}}, "邀请奖励活动一键发放")
		if grantErr != nil {
			_ = s.repo.MarkPayoutItemFailed(ctx, item.ID, grantErr.Error())
			continue
		}
		var after float64
		var before float64
		if len(results) > 0 && results[0].User != nil {
			after = results[0].User.Balance
			before = after - amount
		}
		_ = s.repo.MarkPayoutItemSuccess(ctx, item.ID, before, after)
	}
	batch, err = s.repo.UpdatePayoutBatchSummary(ctx, batch.ID)
	if err != nil {
		return nil, err
	}
	if batch.Status == "success" {
		_, _ = s.repo.UpdateCampaignStatus(ctx, campaignID, CampaignStatusPaid, operatorID)
	}
	return batch, nil
}

func buildInitialCampaignConfig(input CampaignCreateInput) (CampaignConfigVersion, error) {
	cfg := CampaignConfigVersion{
		Version:                  1,
		VersionScope:             CampaignConfigScopePublishSnapshot,
		EffectiveAt:              input.StartAt,
		RechargeThresholdCents:   input.RechargeThresholdCents,
		AllowAccumulatedRecharge: input.AllowAccumulatedRecharge,
		HistoricalInviteRatio:    input.HistoricalInviteRatio,
		PoolInjectionRate:        input.PoolInjectionRate,
		PoolInjectionScope:       normalizedCampaignPoolInjectionScope(input.PoolInjectionScope),
		RankPoolRatio:            input.RankPoolRatio,
		ContributionPoolRatio:    input.ContributionPoolRatio,
		RankRewardCount:          input.RankRewardCount,
		RankWeights:              append([]int64(nil), input.RankWeights...),
		MinPayoutAmountCents:     input.MinPayoutAmountCents,
		PayoutMethod:             "balance",
		PayoutChannel:            "account_balance",
		ChangeReason:             "活动发布快照",
		CreatedBy:                input.OperatorID,
	}
	if cfg.RechargeThresholdCents == 0 {
		cfg.RechargeThresholdCents = 2000
	}
	if cfg.PoolInjectionRate.IsZero() {
		cfg.PoolInjectionRate = decimal.NewFromFloat(0.10)
	}
	cfg.PoolInjectionScope = normalizedCampaignPoolInjectionScope(cfg.PoolInjectionScope)
	if cfg.RankPoolRatio.IsZero() {
		cfg.RankPoolRatio = decimal.NewFromFloat(0.80)
	}
	if cfg.ContributionPoolRatio.IsZero() {
		cfg.ContributionPoolRatio = decimal.NewFromFloat(0.20)
	}
	if cfg.RankRewardCount == 0 {
		cfg.RankRewardCount = 10
	}
	if len(cfg.RankWeights) == 0 {
		cfg.RankWeights = append([]int64(nil), defaultCampaignRankWeights...)
	}
	if cfg.MinPayoutAmountCents == 0 {
		cfg.MinPayoutAmountCents = 100
	}
	return cfg, validateCampaignConfig(cfg)
}

func copiedCampaignName(name string) string {
	base := strings.TrimSpace(name)
	if base == "" {
		base = "Campaign"
	}
	return base + " 副本"
}

func validateCampaignTime(startAt, endAt time.Time) error {
	if startAt.IsZero() || endAt.IsZero() || !endAt.After(startAt) {
		return ErrCampaignInvalidConfig
	}
	return nil
}

func validateCampaignConfig(cfg CampaignConfigVersion) error {
	if cfg.RechargeThresholdCents < 0 || cfg.MinPayoutAmountCents < 0 || cfg.RankRewardCount <= 0 {
		return ErrCampaignInvalidConfig
	}
	if cfg.PoolInjectionRate.IsNegative() || cfg.RankPoolRatio.IsNegative() || cfg.ContributionPoolRatio.IsNegative() ||
		cfg.HistoricalInviteRatio.IsNegative() || cfg.HistoricalInviteRatio.GreaterThan(decimal.NewFromInt(1)) {
		return ErrCampaignInvalidConfig
	}
	if !isCampaignPoolInjectionScope(cfg.PoolInjectionScope) {
		return ErrCampaignInvalidConfig
	}
	if !cfg.RankPoolRatio.Add(cfg.ContributionPoolRatio).Equal(decimal.NewFromInt(1)) {
		return ErrCampaignInvalidConfig
	}
	if len(cfg.RankWeights) < cfg.RankRewardCount {
		return ErrCampaignInvalidConfig
	}
	var total int64
	for i := 0; i < cfg.RankRewardCount; i++ {
		if cfg.RankWeights[i] < 0 {
			return ErrCampaignInvalidConfig
		}
		total += cfg.RankWeights[i]
	}
	if total != 100 {
		return ErrCampaignInvalidConfig
	}
	return nil
}

func normalizedCampaignPoolInjectionScope(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return CampaignPoolInjectionScopeInviteesOnly
	}
	return scope
}

func isCampaignPoolInjectionScope(scope string) bool {
	switch normalizedCampaignPoolInjectionScope(scope) {
	case CampaignPoolInjectionScopeInviteesOnly, CampaignPoolInjectionScopeAllUsers:
		return true
	default:
		return false
	}
}

func calculatePoolInjectionCents(rechargeAmountCents int64, rate decimal.Decimal) int64 {
	if rechargeAmountCents <= 0 || rate.IsNegative() {
		return 0
	}
	return decimal.NewFromInt(rechargeAmountCents).Mul(rate).Floor().IntPart()
}

func calculateCampaignRewards(campaignID int64, cfg *CampaignConfigVersion, finalPoolCents int64, rows []CampaignLeaderboardRow, status, batchNo string) *CampaignCalculationSummary {
	if cfg == nil {
		return &CampaignCalculationSummary{CampaignID: campaignID, CalculationStatus: status, CalculationBatchNo: batchNo}
	}
	sortedRows := append([]CampaignLeaderboardRow(nil), rows...)
	sort.SliceStable(sortedRows, func(i, j int) bool {
		if sortedRows[i].ValidInviteCount != sortedRows[j].ValidInviteCount {
			return sortedRows[i].ValidInviteCount > sortedRows[j].ValidInviteCount
		}
		if sortedRows[i].InviteeRechargeAmountCents != sortedRows[j].InviteeRechargeAmountCents {
			return sortedRows[i].InviteeRechargeAmountCents > sortedRows[j].InviteeRechargeAmountCents
		}
		if !sortedRows[i].ReachedCountAt.Equal(sortedRows[j].ReachedCountAt) {
			return sortedRows[i].ReachedCountAt.Before(sortedRows[j].ReachedCountAt)
		}
		return sortedRows[i].JoinedAt.Before(sortedRows[j].JoinedAt)
	})
	rankPool := decimal.NewFromInt(finalPoolCents).Mul(cfg.RankPoolRatio).Floor().IntPart()
	contributionPool := decimal.NewFromInt(finalPoolCents).Mul(cfg.ContributionPoolRatio).Floor().IntPart()
	rankWinnerCount := min(len(sortedRows), cfg.RankRewardCount, len(cfg.RankWeights))
	var rankWeightSum int64
	for _, weight := range cfg.RankWeights[:rankWinnerCount] {
		rankWeightSum += weight
	}
	weightSum := decimal.Zero
	for _, row := range sortedRows {
		if row.ValidInviteCount > 0 {
			weightSum = weightSum.Add(decimal.NewFromFloat(math.Sqrt(row.ValidInviteCount)))
		}
	}
	results := make([]CampaignRewardResult, 0, len(sortedRows))
	for i, row := range sortedRows {
		rank := i + 1
		var rankPtr *int
		rankPtr = &rank
		rankReward := int64(0)
		if rank <= rankWinnerCount && rankWeightSum > 0 {
			rankReward = decimal.NewFromInt(rankPool).Mul(decimal.NewFromInt(cfg.RankWeights[rank-1])).Div(decimal.NewFromInt(rankWeightSum)).Floor().IntPart()
		}
		contributionWeight := decimal.Zero
		contributionReward := int64(0)
		if row.ValidInviteCount > 0 && !weightSum.IsZero() {
			contributionWeight = decimal.NewFromFloat(math.Sqrt(row.ValidInviteCount))
			contributionReward = decimal.NewFromInt(contributionPool).Mul(contributionWeight).Div(weightSum).Floor().IntPart()
		}
		gross := rankReward + contributionReward
		finalPayout := gross
		withheld := int64(0)
		withheldReason := ""
		if gross > 0 && gross < cfg.MinPayoutAmountCents {
			withheld = gross
			finalPayout = 0
			withheldReason = "低于最低发放金额未发放"
		}
		result := CampaignRewardResult{
			CampaignID:                    campaignID,
			ConfigVersionID:               cfg.ID,
			CalculationBatchNo:            batchNo,
			UserID:                        row.UserID,
			Rank:                          rankPtr,
			RankRewardAmountCents:         rankReward,
			ContributionWeight:            contributionWeight,
			ContributionRewardAmountCents: contributionReward,
			GrossRewardAmountCents:        gross,
			MinPayoutAmountSnapshotCents:  cfg.MinPayoutAmountCents,
			FinalPayoutAmountCents:        finalPayout,
			WithheldAmountCents:           withheld,
			WithheldReason:                withheldReason,
			CalculationStatus:             status,
			CalculatedAt:                  time.Now(),
		}
		results = append(results, result)
	}
	var totalGross int64
	for _, result := range results {
		totalGross += result.GrossRewardAmountCents
	}
	if residual := finalPoolCents - totalGross; residual > 0 && len(results) > 0 {
		results[0].RoundingResidualCents += residual
	}
	summary := &CampaignCalculationSummary{
		CampaignID:            campaignID,
		CalculationStatus:     status,
		CalculationBatchNo:    batchNo,
		FinalPoolCents:        finalPoolCents,
		RankPoolCents:         rankPool,
		ContributionPoolCents: contributionPool,
		Results:               results,
	}
	for _, result := range results {
		summary.TotalGrossRewardCents += result.GrossRewardAmountCents
		summary.TotalFinalPayoutCents += result.FinalPayoutAmountCents
		summary.TotalWithheldCents += result.WithheldAmountCents
		summary.TotalRoundingResidualCents += result.RoundingResidualCents
	}
	return summary
}

func summarizeCampaignRewardResults(campaignID int64, cfg *CampaignConfigVersion, finalPoolCents int64, status string, results []CampaignRewardResult) *CampaignCalculationSummary {
	rankPoolCents := int64(0)
	contributionPoolCents := int64(0)
	if cfg != nil {
		rankPoolCents = decimal.NewFromInt(finalPoolCents).Mul(cfg.RankPoolRatio).Floor().IntPart()
		contributionPoolCents = decimal.NewFromInt(finalPoolCents).Mul(cfg.ContributionPoolRatio).Floor().IntPart()
	}
	batchNo := ""
	if len(results) > 0 {
		batchNo = results[0].CalculationBatchNo
	}
	summary := &CampaignCalculationSummary{
		CampaignID:            campaignID,
		CalculationStatus:     status,
		CalculationBatchNo:    batchNo,
		FinalPoolCents:        finalPoolCents,
		RankPoolCents:         rankPoolCents,
		ContributionPoolCents: contributionPoolCents,
		Results:               results,
	}
	for _, result := range results {
		summary.TotalGrossRewardCents += result.GrossRewardAmountCents
		summary.TotalFinalPayoutCents += result.FinalPayoutAmountCents
		summary.TotalWithheldCents += result.WithheldAmountCents
		summary.TotalRoundingResidualCents += result.RoundingResidualCents
	}
	return summary
}

func centsFromYuan(amount float64) int64 {
	if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0
	}
	return decimal.NewFromFloat(amount).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
}

func centsToYuanFloat(cents int64) float64 {
	v, _ := decimal.NewFromInt(cents).Div(decimal.NewFromInt(100)).Float64()
	return v
}

func campaignConfigWeightsJSON(weights []int64) string {
	b, _ := json.Marshal(weights)
	return string(b)
}

func parseCampaignWeights(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return append([]int64(nil), defaultCampaignRankWeights...)
	}
	var result []int64
	if err := json.Unmarshal([]byte(raw), &result); err == nil && len(result) > 0 {
		return result
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == '/' || r == ',' || r == ' ' })
	for _, part := range parts {
		v, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err == nil {
			result = append(result, v)
		}
	}
	if len(result) == 0 {
		return append([]int64(nil), defaultCampaignRankWeights...)
	}
	return result
}

func maskCampaignEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.IndexByte(email, '@')
	if at <= 1 {
		if email == "" {
			return ""
		}
		return "***"
	}
	return email[:1] + "***" + email[at:]
}
