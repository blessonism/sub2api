package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

type campaignRepository struct {
	db *sql.DB
}

func NewCampaignRepository(db *sql.DB) service.CampaignRepository {
	return &campaignRepository{db: db}
}

func (r *campaignRepository) CreateCampaign(ctx context.Context, input service.CampaignCreateInput, cfg service.CampaignConfigVersion) (*service.Campaign, *service.CampaignConfigVersion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	campaign := &service.Campaign{}
	err = tx.QueryRowContext(ctx, `
INSERT INTO campaigns (
	name, description, cover_url, rules_text, status,
	warmup_start_at, start_at, end_at, audit_start_at, audit_end_at,
	publicity_start_at, publicity_end_at, payout_due_at, created_by, updated_by,
	created_at, updated_at
) VALUES (
	$1, $2, $3, $4, 'draft',
	$5, $6, $7, $8, $9,
	$10, $11, $12, $13, $13,
	NOW(), NOW()
)
RETURNING id, name, description, cover_url, rules_text, status, warmup_start_at, start_at, end_at,
	audit_start_at, audit_end_at, publicity_start_at, publicity_end_at, payout_due_at,
	published_config_version_id, created_by, updated_by, created_at, updated_at`,
		strings.TrimSpace(input.Name), input.Description, input.CoverURL, input.RulesText,
		input.WarmupStartAt, input.StartAt, input.EndAt, input.AuditStartAt, input.AuditEndAt,
		input.PublicityStartAt, input.PublicityEndAt, input.PayoutDueAt, nullableInt64(input.OperatorID),
	).Scan(
		&campaign.ID, &campaign.Name, &campaign.Description, &campaign.CoverURL, &campaign.RulesText, &campaign.Status,
		&campaign.WarmupStartAt, &campaign.StartAt, &campaign.EndAt, &campaign.AuditStartAt, &campaign.AuditEndAt,
		&campaign.PublicityStartAt, &campaign.PublicityEndAt, &campaign.PayoutDueAt, &campaign.PublishedConfigVersionID,
		&campaign.CreatedBy, &campaign.UpdatedBy, &campaign.CreatedAt, &campaign.UpdatedAt,
	)
	if err != nil {
		return nil, nil, err
	}
	cfg.CampaignID = campaign.ID
	cfg.Version = 1
	version, err := insertCampaignConfigVersion(ctx, tx, cfg)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return campaign, version, nil
}

func (r *campaignRepository) UpdateCampaign(ctx context.Context, campaignID int64, input service.CampaignUpdateInput) (*service.Campaign, error) {
	current, err := r.GetCampaign(ctx, campaignID)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		current.Name = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		current.Description = *input.Description
	}
	if input.CoverURL != nil {
		current.CoverURL = *input.CoverURL
	}
	if input.RulesText != nil {
		current.RulesText = *input.RulesText
	}
	if input.StartAt != nil {
		current.StartAt = *input.StartAt
	}
	if input.EndAt != nil {
		current.EndAt = *input.EndAt
	}
	_, err = r.db.ExecContext(ctx, `
UPDATE campaigns
SET name = $2, description = $3, cover_url = $4, rules_text = $5,
	start_at = $6, end_at = $7, updated_by = $8, updated_at = NOW()
WHERE id = $1`,
		campaignID, current.Name, current.Description, current.CoverURL, current.RulesText,
		current.StartAt, current.EndAt, nullableInt64(input.OperatorID),
	)
	if err != nil {
		return nil, err
	}
	return r.GetCampaign(ctx, campaignID)
}

func (r *campaignRepository) GetCampaign(ctx context.Context, campaignID int64) (*service.Campaign, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, name, description, cover_url, rules_text, status, warmup_start_at, start_at, end_at,
	audit_start_at, audit_end_at, publicity_start_at, publicity_end_at, payout_due_at,
	published_config_version_id, created_by, updated_by, created_at, updated_at
FROM campaigns
WHERE id = $1`, campaignID)
	campaign, err := scanCampaign(row)
	if err != nil {
		return nil, campaignRepoErr(err)
	}
	return campaign, nil
}

func (r *campaignRepository) ListCampaigns(ctx context.Context, page, pageSize int) ([]service.Campaign, int64, error) {
	offset := (page - 1) * pageSize
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM campaigns`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, description, cover_url, rules_text, status, warmup_start_at, start_at, end_at,
	audit_start_at, audit_end_at, publicity_start_at, publicity_end_at, payout_due_at,
	published_config_version_id, created_by, updated_by, created_at, updated_at
FROM campaigns
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.Campaign, 0)
	for rows.Next() {
		item, err := scanCampaign(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (r *campaignRepository) GetActiveCampaign(ctx context.Context, now time.Time) (*service.Campaign, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, name, description, cover_url, rules_text, status, warmup_start_at, start_at, end_at,
	audit_start_at, audit_end_at, publicity_start_at, publicity_end_at, payout_due_at,
	published_config_version_id, created_by, updated_by, created_at, updated_at
FROM campaigns
WHERE status = 'active'
  AND start_at <= $1
  AND end_at > $1
ORDER BY start_at DESC
LIMIT 1`, now)
	campaign, err := scanCampaign(row)
	if err != nil {
		return nil, campaignRepoErr(err)
	}
	return campaign, nil
}

func (r *campaignRepository) CreateConfigVersion(ctx context.Context, campaignID int64, _ service.CampaignConfigVersionInput, cfg service.CampaignConfigVersion) (*service.CampaignConfigVersion, error) {
	var nextVersion int
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) + 1 FROM campaign_config_versions WHERE campaign_id = $1`, campaignID).Scan(&nextVersion); err != nil {
		return nil, err
	}
	cfg.CampaignID = campaignID
	cfg.Version = nextVersion
	return insertCampaignConfigVersion(ctx, r.db, cfg)
}

func (r *campaignRepository) GetLatestConfigVersionAt(ctx context.Context, campaignID int64, at time.Time) (*service.CampaignConfigVersion, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, campaign_id, version, version_scope, effective_at, recharge_threshold_cents,
	allow_accumulated_recharge, pool_injection_rate::text, rank_pool_ratio::text,
	contribution_pool_ratio::text, rank_reward_count, rank_weights_json::text,
	min_payout_amount_cents, payout_method, payout_channel, change_reason, created_by, created_at
FROM campaign_config_versions
WHERE campaign_id = $1 AND effective_at <= $2
ORDER BY effective_at DESC, version DESC
LIMIT 1`, campaignID, at)
	cfg, err := scanCampaignConfigVersion(row)
	if err != nil && errors.Is(err, service.ErrCampaignNotFound) {
		row = r.db.QueryRowContext(ctx, `
SELECT id, campaign_id, version, version_scope, effective_at, recharge_threshold_cents,
	allow_accumulated_recharge, pool_injection_rate::text, rank_pool_ratio::text,
	contribution_pool_ratio::text, rank_reward_count, rank_weights_json::text,
	min_payout_amount_cents, payout_method, payout_channel, change_reason, created_by, created_at
FROM campaign_config_versions
WHERE campaign_id = $1
ORDER BY version DESC
LIMIT 1`, campaignID)
		cfg, err = scanCampaignConfigVersion(row)
	}
	return cfg, err
}

func (r *campaignRepository) GetPublishedConfigVersion(ctx context.Context, campaignID int64) (*service.CampaignConfigVersion, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT cv.id, cv.campaign_id, cv.version, cv.version_scope, cv.effective_at, cv.recharge_threshold_cents,
	cv.allow_accumulated_recharge, cv.pool_injection_rate::text, cv.rank_pool_ratio::text,
	cv.contribution_pool_ratio::text, cv.rank_reward_count, cv.rank_weights_json::text,
	cv.min_payout_amount_cents, cv.payout_method, cv.payout_channel, cv.change_reason, cv.created_by, cv.created_at
FROM campaigns c
JOIN campaign_config_versions cv ON cv.id = c.published_config_version_id
WHERE c.id = $1`, campaignID)
	return scanCampaignConfigVersion(row)
}

func (r *campaignRepository) PublishCampaign(ctx context.Context, campaignID int64, operatorID *int64, now time.Time) (*service.Campaign, error) {
	status := service.CampaignStatusActive
	var publishedID int64
	if err := r.db.QueryRowContext(ctx, `
SELECT id
FROM campaign_config_versions
WHERE campaign_id = $1
ORDER BY version ASC
LIMIT 1`, campaignID).Scan(&publishedID); err != nil {
		return nil, campaignRepoErr(err)
	}
	if _, err := r.db.ExecContext(ctx, `
UPDATE campaigns
SET status = $2,
	published_config_version_id = COALESCE(published_config_version_id, $3),
	updated_by = $4,
	updated_at = NOW()
WHERE id = $1`, campaignID, status, publishedID, nullableInt64(operatorID)); err != nil {
		if strings.Contains(err.Error(), "campaigns_single_active_idx") {
			return nil, service.ErrCampaignDuplicateActive
		}
		return nil, err
	}
	return r.GetCampaign(ctx, campaignID)
}

func (r *campaignRepository) UpdateCampaignStatus(ctx context.Context, campaignID int64, status string, operatorID *int64) (*service.Campaign, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE campaigns SET status = $2, updated_by = $3, updated_at = NOW() WHERE id = $1`, campaignID, status, nullableInt64(operatorID))
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, service.ErrCampaignNotFound
	}
	return r.GetCampaign(ctx, campaignID)
}

func (r *campaignRepository) HasActiveCampaign(ctx context.Context, excludeCampaignID int64) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM campaigns WHERE status = 'active' AND id <> $1`, excludeCampaignID).Scan(&count)
	return count > 0, err
}

func (r *campaignRepository) GetInviterByAffiliateCode(ctx context.Context, code string) (*service.AffiliateSummary, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT user_id, aff_code, aff_code_custom, aff_rebate_rate_percent::double precision,
	inviter_id, aff_count, aff_quota::double precision, aff_frozen_quota::double precision,
	aff_history_quota::double precision, created_at, updated_at
FROM user_affiliates
WHERE UPPER(aff_code) = UPPER($1)
LIMIT 1`, strings.TrimSpace(code))
	var item service.AffiliateSummary
	if err := row.Scan(
		&item.UserID, &item.AffCode, &item.AffCodeCustom, &item.AffRebateRatePercent,
		&item.InviterID, &item.AffCount, &item.AffQuota, &item.AffFrozenQuota,
		&item.AffHistoryQuota, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, campaignRepoErr(err)
	}
	return &item, nil
}

func (r *campaignRepository) RecordInviteRegistration(ctx context.Context, campaign *service.Campaign, cfg *service.CampaignConfigVersion, inviter *service.AffiliateSummary, input service.CampaignRegisterInviteInput) (*service.CampaignInviteRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureCampaignParticipant(ctx, tx, campaign.ID, inviter.UserID, inviter.AffCode); err != nil {
		return nil, err
	}
	row := tx.QueryRowContext(ctx, `
INSERT INTO campaign_invite_records (
	campaign_id, config_version_id, inviter_user_id, invitee_user_id, invite_source,
	threshold_snapshot_cents, registered_at, status, risk_level, audit_status, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, 'registered', 'low', 'not_reviewed', NOW(), NOW())
ON CONFLICT (campaign_id, invitee_user_id) DO UPDATE
SET updated_at = campaign_invite_records.updated_at
RETURNING id, campaign_id, config_version_id, inviter_user_id, invitee_user_id, invite_source,
	threshold_snapshot_cents, registered_at, qualified_at, effective_recharge_amount_cents,
	status, risk_level, invalid_reason, audit_status, audit_by, audit_at, audit_note`,
		campaign.ID, cfg.ID, inviter.UserID, input.InviteeUserID, input.InviteSource,
		cfg.RechargeThresholdCents, input.RegisteredAt,
	)
	record, err := scanCampaignInviteRecord(row)
	if err != nil {
		return nil, err
	}
	if err := refreshCampaignParticipantStats(ctx, tx, campaign.ID, inviter.UserID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return record, nil
}

func (r *campaignRepository) RecordRecharge(ctx context.Context, campaign *service.Campaign, _ *service.CampaignConfigVersion, input service.CampaignRechargeInput) (*service.CampaignInviteRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	row := tx.QueryRowContext(ctx, `
UPDATE campaign_invite_records
SET effective_recharge_amount_cents = effective_recharge_amount_cents + $3,
	status = CASE
		WHEN effective_recharge_amount_cents + $3 >= threshold_snapshot_cents THEN 'effective'
		ELSE 'recharge_unqualified'
	END,
	audit_status = CASE
		WHEN effective_recharge_amount_cents + $3 >= threshold_snapshot_cents THEN 'approved'
		ELSE audit_status
	END,
	qualified_at = CASE
		WHEN qualified_at IS NULL AND effective_recharge_amount_cents + $3 >= threshold_snapshot_cents THEN $4
		ELSE qualified_at
	END,
	updated_at = NOW()
WHERE campaign_id = $1
  AND invitee_user_id = $2
  AND registered_at >= $5
  AND registered_at < $6
RETURNING id, campaign_id, config_version_id, inviter_user_id, invitee_user_id, invite_source,
	threshold_snapshot_cents, registered_at, qualified_at, effective_recharge_amount_cents,
	status, risk_level, invalid_reason, audit_status, audit_by, audit_at, audit_note`,
		campaign.ID, input.InviteeUserID, input.RechargeAmountCents, input.SourceSuccessAt, campaign.StartAt, campaign.EndAt,
	)
	record, err := scanCampaignInviteRecord(row)
	if err != nil {
		return nil, campaignRepoErr(err)
	}
	if err := refreshCampaignParticipantStats(ctx, tx, campaign.ID, record.InviterUserID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return record, nil
}

func (r *campaignRepository) ListInviteRecords(ctx context.Context, campaignID, inviterUserID int64, page, pageSize int) ([]service.CampaignInviteRecord, int64, error) {
	offset := (page - 1) * pageSize
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM campaign_invite_records WHERE campaign_id = $1 AND inviter_user_id = $2`, campaignID, inviterUserID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT cir.id, cir.campaign_id, cir.config_version_id, cir.inviter_user_id, cir.invitee_user_id, cir.invite_source,
	cir.threshold_snapshot_cents, cir.registered_at, cir.qualified_at, cir.effective_recharge_amount_cents,
	cir.status, cir.risk_level, cir.invalid_reason, cir.audit_status, cir.audit_by, cir.audit_at, cir.audit_note,
	COALESCE(u.email, ''), COALESCE(u.username, '')
FROM campaign_invite_records cir
LEFT JOIN users u ON u.id = cir.invitee_user_id
WHERE cir.campaign_id = $1 AND cir.inviter_user_id = $2
ORDER BY cir.registered_at DESC, cir.id DESC
LIMIT $3 OFFSET $4`, campaignID, inviterUserID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.CampaignInviteRecord, 0)
	for rows.Next() {
		item, err := scanCampaignInviteRecordWithUser(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func (r *campaignRepository) GetParticipantStats(ctx context.Context, campaignID, userID int64) (*service.CampaignParticipant, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT cp.campaign_id, cp.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''),
	cp.invite_code_snapshot, cp.invite_link_snapshot, cp.participant_status, cp.joined_at,
	cp.valid_invite_count, cp.pending_invite_count, cp.invalid_invite_count,
	cp.invitee_recharge_amount_cents, cp.estimated_rank_reward_cents,
	cp.estimated_contribution_reward_cents, cp.estimated_total_reward_cents,
	cp.final_rank_reward_cents, cp.final_contribution_reward_cents, cp.final_total_reward_cents
FROM campaign_participants cp
LEFT JOIN users u ON u.id = cp.user_id
WHERE cp.campaign_id = $1 AND cp.user_id = $2`, campaignID, userID)
	var item service.CampaignParticipant
	var email string
	if err := row.Scan(
		&item.CampaignID, &item.UserID, &email, &item.Username,
		&item.InviteCodeSnapshot, &item.InviteLinkSnapshot, &item.ParticipantStatus, &item.JoinedAt,
		&item.ValidInviteCount, &item.PendingInviteCount, &item.InvalidInviteCount,
		&item.InviteeRechargeAmountCents, &item.EstimatedRankRewardCents,
		&item.EstimatedContributionRewardCents, &item.EstimatedTotalRewardCents,
		&item.FinalRankRewardCents, &item.FinalContributionRewardCents, &item.FinalTotalRewardCents,
	); err != nil {
		return nil, campaignRepoErr(err)
	}
	item.MaskedEmail = maskEmail(email)
	return &item, nil
}

func (r *campaignRepository) ListLeaderboardRows(ctx context.Context, campaignID int64, limit int) ([]service.CampaignLeaderboardRow, error) {
	rows, err := r.db.QueryContext(ctx, campaignLeaderboardSQL()+` LIMIT $2`, campaignID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanCampaignLeaderboardRows(rows)
}

func (r *campaignRepository) ListRewardEligibleRows(ctx context.Context, campaignID int64) ([]service.CampaignLeaderboardRow, error) {
	rows, err := r.db.QueryContext(ctx, campaignLeaderboardSQL(), campaignID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanCampaignLeaderboardRows(rows)
}

func (r *campaignRepository) InsertPoolEntry(ctx context.Context, campaign *service.Campaign, cfg *service.CampaignConfigVersion, invite *service.CampaignInviteRecord, input service.CampaignRechargeInput, poolAmountCents int64) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
INSERT INTO campaign_pool_entries (
	campaign_id, config_version_id, invite_record_id, invitee_user_id, source_type, source_id,
	source_success_at, effective_recharge_amount_cents, injection_rate_snapshot, pool_amount_cents,
	pool_status, confirmed_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::numeric, $10, 'confirmed', NOW(), NOW(), NOW())
ON CONFLICT (campaign_id, source_type, source_id) DO NOTHING`,
		campaign.ID, cfg.ID, invite.ID, input.InviteeUserID, input.SourceType, input.SourceID,
		input.SourceSuccessAt, input.RechargeAmountCents, cfg.PoolInjectionRate.String(), poolAmountCents,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *campaignRepository) AddPoolAdjustment(ctx context.Context, campaignID int64, input service.CampaignPoolAdjustmentInput) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO campaign_pool_adjustments (campaign_id, adjustment_type, amount_cents, reason, operator_id, created_at)
VALUES ($1, $2, $3, $4, $5, NOW())`, campaignID, input.AdjustmentType, input.AmountCents, input.Reason, nullableInt64(input.OperatorID))
	return err
}

func (r *campaignRepository) GetPoolSummary(ctx context.Context, campaignID int64) (*service.CampaignPoolSummary, error) {
	summary := &service.CampaignPoolSummary{CampaignID: campaignID}
	err := r.db.QueryRowContext(ctx, `
SELECT
	COALESCE(SUM(CASE WHEN pool_status = 'confirmed' THEN GREATEST(pool_amount_cents - deducted_amount_cents, 0) ELSE 0 END), 0),
	COALESCE(SUM(CASE WHEN pool_status = 'pending' THEN GREATEST(pool_amount_cents - deducted_amount_cents, 0) ELSE 0 END), 0),
	COALESCE(SUM(deducted_amount_cents), 0)
FROM campaign_pool_entries
WHERE campaign_id = $1`, campaignID).Scan(&summary.ConfirmedPoolCents, &summary.PendingPoolCents, &summary.DeductedPoolCents)
	if err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents), 0) FROM campaign_pool_adjustments WHERE campaign_id = $1`, campaignID).Scan(&summary.AdjustmentTotalCents); err != nil {
		return nil, err
	}
	summary.EstimatedTotalPoolCents = summary.ConfirmedPoolCents + summary.PendingPoolCents + summary.AdjustmentTotalCents
	summary.FinalPoolCents = summary.EstimatedTotalPoolCents
	if summary.FinalPoolCents < 0 {
		summary.FinalPoolCents = 0
	}
	return summary, nil
}

func (r *campaignRepository) InsertPoolDeduction(ctx context.Context, input service.CampaignDeductionInput) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `
INSERT INTO campaign_pool_deductions (
	campaign_id, pool_entry_id, source_type, source_id, deduct_amount_cents,
	idempotency_key, reason, processed_at, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
ON CONFLICT (idempotency_key) DO NOTHING`,
		input.CampaignID, input.PoolEntryID, input.SourceType, input.SourceID, input.DeductAmountCents, input.IdempotencyKey, input.Reason,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, tx.Commit()
	}
	_, err = tx.ExecContext(ctx, `
UPDATE campaign_pool_entries
SET deducted_amount_cents = deducted_amount_cents + $2,
	pool_status = CASE WHEN deducted_amount_cents + $2 >= pool_amount_cents THEN 'deducted' ELSE pool_status END,
	last_deduct_reason = $3,
	updated_at = NOW()
WHERE id = $1`, input.PoolEntryID, input.DeductAmountCents, input.Reason)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (r *campaignRepository) SaveLeaderboardSnapshot(ctx context.Context, campaignID int64, snapshotType string, rows []service.CampaignLeaderboardRow) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM campaign_leaderboard_snapshots WHERE campaign_id = $1 AND snapshot_type = $2`, campaignID, snapshotType); err != nil {
		return err
	}
	for _, row := range rows {
		_, err := tx.ExecContext(ctx, `
INSERT INTO campaign_leaderboard_snapshots (
	campaign_id, snapshot_type, rank, user_id, valid_invite_count, invitee_recharge_amount_cents,
	reached_count_at, joined_at, estimated_reward_cents, final_reward_cents, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())`,
			campaignID, snapshotType, row.Rank, row.UserID, row.ValidInviteCount, row.InviteeRechargeAmountCents,
			row.ReachedCountAt, row.JoinedAt, row.EstimatedRewardCents, row.FinalRewardCents,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *campaignRepository) SaveRewardResults(ctx context.Context, campaignID int64, status string, batchNo string, results []service.CampaignRewardResult) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM campaign_reward_results WHERE campaign_id = $1 AND calculation_status = $2`, campaignID, status); err != nil {
		return err
	}
	for _, result := range results {
		rank := any(nil)
		if result.Rank != nil {
			rank = *result.Rank
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO campaign_reward_results (
	campaign_id, config_version_id, calculation_batch_no, user_id, rank,
	rank_reward_amount_cents, contribution_weight, contribution_reward_amount_cents, gross_reward_amount_cents,
	min_payout_amount_snapshot_cents, final_payout_amount_cents, withheld_amount_cents, withheld_reason,
	rounding_residual_cents, calculation_status, calculated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7::numeric, $8, $9, $10, $11, $12, $13, $14, $15, NOW())`,
			campaignID, result.ConfigVersionID, batchNo, result.UserID, rank,
			result.RankRewardAmountCents, result.ContributionWeight.String(), result.ContributionRewardAmountCents,
			result.GrossRewardAmountCents, result.MinPayoutAmountSnapshotCents, result.FinalPayoutAmountCents,
			result.WithheldAmountCents, result.WithheldReason, result.RoundingResidualCents, status,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *campaignRepository) ListRewardResults(ctx context.Context, campaignID int64, status string) ([]service.CampaignRewardResult, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, campaign_id, config_version_id, calculation_batch_no, user_id, rank,
	rank_reward_amount_cents, contribution_weight::text, contribution_reward_amount_cents,
	gross_reward_amount_cents, min_payout_amount_snapshot_cents, final_payout_amount_cents,
	withheld_amount_cents, withheld_reason, rounding_residual_cents, calculation_status, calculated_at
FROM campaign_reward_results
WHERE campaign_id = $1 AND calculation_status = $2
ORDER BY COALESCE(rank, 999999), final_payout_amount_cents DESC`, campaignID, status)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	results := make([]service.CampaignRewardResult, 0)
	for rows.Next() {
		item, err := scanCampaignRewardResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *item)
	}
	return results, rows.Err()
}

func (r *campaignRepository) CreatePayoutBatch(ctx context.Context, campaignID int64, batchNo string, operatorID *int64, results []service.CampaignRewardResult) (*service.CampaignPayoutBatch, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var totalAmount int64
	for _, result := range results {
		totalAmount += result.FinalPayoutAmountCents
	}
	var batchID int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO campaign_payout_batches (
	campaign_id, batch_no, status, operator_id, total_users, total_amount_cents, started_at, created_at
) VALUES ($1, $2, 'processing', $3, $4, $5, NOW(), NOW())
RETURNING id`, campaignID, batchNo, nullableInt64(operatorID), len(results), totalAmount).Scan(&batchID)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		key := fmt.Sprintf("campaign:%d:payout:%d:%s", campaignID, result.UserID, batchNo)
		_, err := tx.ExecContext(ctx, `
INSERT INTO campaign_payout_items (
	batch_id, campaign_id, user_id, reward_result_id, amount_cents, status, idempotency_key, created_at
) VALUES ($1, $2, $3, $4, $5, 'pending', $6, NOW())
ON CONFLICT (idempotency_key) DO NOTHING`,
			batchID, campaignID, result.UserID, result.ID, result.FinalPayoutAmountCents, key,
		)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.UpdatePayoutBatchSummary(ctx, batchID)
}

func (r *campaignRepository) MarkPayoutItemSuccess(ctx context.Context, itemID int64, before, after float64) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE campaign_payout_items
SET status = 'success', balance_before_snapshot = $2, balance_after_snapshot = $3,
	error_message = '', processed_at = NOW()
WHERE id = $1 AND status <> 'success'`, itemID, before, after)
	return err
}

func (r *campaignRepository) MarkPayoutItemFailed(ctx context.Context, itemID int64, errMessage string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE campaign_payout_items
SET status = 'failed', error_message = $2, processed_at = NOW()
WHERE id = $1 AND status <> 'success'`, itemID, errMessage)
	return err
}

func (r *campaignRepository) ListPayoutItems(ctx context.Context, batchID int64) ([]service.CampaignPayoutItem, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, batch_id, campaign_id, user_id, reward_result_id, amount_cents,
	balance_before_snapshot::double precision, balance_after_snapshot::double precision,
	status, idempotency_key, error_message, processed_at, created_at
FROM campaign_payout_items
WHERE batch_id = $1
ORDER BY id`, batchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.CampaignPayoutItem, 0)
	for rows.Next() {
		var item service.CampaignPayoutItem
		if err := rows.Scan(
			&item.ID, &item.BatchID, &item.CampaignID, &item.UserID, &item.RewardResultID, &item.AmountCents,
			&item.BalanceBeforeSnapshot, &item.BalanceAfterSnapshot, &item.Status, &item.IdempotencyKey,
			&item.ErrorMessage, &item.ProcessedAt, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *campaignRepository) UpdatePayoutBatchSummary(ctx context.Context, batchID int64) (*service.CampaignPayoutBatch, error) {
	_, err := r.db.ExecContext(ctx, `
WITH item_stats AS (
	SELECT
		COUNT(*)::integer AS total_users,
		COALESCE(SUM(amount_cents), 0)::bigint AS total_amount,
		COUNT(*) FILTER (WHERE status = 'success')::integer AS success_count,
		COUNT(*) FILTER (WHERE status = 'failed')::integer AS failed_count,
		COUNT(*) FILTER (WHERE status = 'pending')::integer AS pending_count
	FROM campaign_payout_items
	WHERE batch_id = $1
)
UPDATE campaign_payout_batches b
SET total_users = s.total_users,
	total_amount_cents = s.total_amount,
	success_count = s.success_count,
	failed_count = s.failed_count,
	status = CASE
		WHEN s.failed_count = 0 AND s.pending_count = 0 THEN 'success'
		WHEN s.success_count > 0 THEN 'partial_success'
		WHEN s.failed_count > 0 AND s.pending_count = 0 THEN 'failed'
		ELSE 'processing'
	END,
	finished_at = CASE WHEN s.pending_count = 0 THEN NOW() ELSE finished_at END
FROM item_stats s
WHERE b.id = $1`, batchID)
	if err != nil {
		return nil, err
	}
	return r.getPayoutBatch(ctx, batchID)
}

func (r *campaignRepository) GetSuccessfulPayoutBatch(ctx context.Context, campaignID int64) (*service.CampaignPayoutBatch, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, campaign_id, batch_no, status, operator_id, total_users, total_amount_cents,
	success_count, failed_count, started_at, finished_at, created_at
FROM campaign_payout_batches
WHERE campaign_id = $1 AND status IN ('processing', 'partial_success', 'success')
ORDER BY id DESC
LIMIT 1`, campaignID)
	batch, err := scanCampaignPayoutBatch(row)
	if err != nil {
		return nil, campaignRepoErr(err)
	}
	return batch, nil
}

func (r *campaignRepository) getPayoutBatch(ctx context.Context, batchID int64) (*service.CampaignPayoutBatch, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, campaign_id, batch_no, status, operator_id, total_users, total_amount_cents,
	success_count, failed_count, started_at, finished_at, created_at
FROM campaign_payout_batches
WHERE id = $1`, batchID)
	return scanCampaignPayoutBatch(row)
}

type campaignScanner interface {
	Scan(dest ...any) error
}

func scanCampaign(scanner campaignScanner) (*service.Campaign, error) {
	var item service.Campaign
	err := scanner.Scan(
		&item.ID, &item.Name, &item.Description, &item.CoverURL, &item.RulesText, &item.Status,
		&item.WarmupStartAt, &item.StartAt, &item.EndAt, &item.AuditStartAt, &item.AuditEndAt,
		&item.PublicityStartAt, &item.PublicityEndAt, &item.PayoutDueAt, &item.PublishedConfigVersionID,
		&item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func insertCampaignConfigVersion(ctx context.Context, execer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, cfg service.CampaignConfigVersion) (*service.CampaignConfigVersion, error) {
	row := execer.QueryRowContext(ctx, `
INSERT INTO campaign_config_versions (
	campaign_id, version, version_scope, effective_at, recharge_threshold_cents,
	allow_accumulated_recharge, pool_injection_rate, rank_pool_ratio, contribution_pool_ratio,
	rank_reward_count, rank_weights_json, min_payout_amount_cents, payout_method, payout_channel,
	change_reason, created_by, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7::numeric, $8::numeric, $9::numeric, $10, $11::jsonb, $12, $13, $14, $15, $16, NOW())
RETURNING id, campaign_id, version, version_scope, effective_at, recharge_threshold_cents,
	allow_accumulated_recharge, pool_injection_rate::text, rank_pool_ratio::text,
	contribution_pool_ratio::text, rank_reward_count, rank_weights_json::text,
	min_payout_amount_cents, payout_method, payout_channel, change_reason, created_by, created_at`,
		cfg.CampaignID, cfg.Version, cfg.VersionScope, cfg.EffectiveAt, cfg.RechargeThresholdCents,
		cfg.AllowAccumulatedRecharge, cfg.PoolInjectionRate.String(), cfg.RankPoolRatio.String(),
		cfg.ContributionPoolRatio.String(), cfg.RankRewardCount, weightsJSON(cfg.RankWeights),
		cfg.MinPayoutAmountCents, cfg.PayoutMethod, cfg.PayoutChannel, cfg.ChangeReason, nullableInt64(cfg.CreatedBy),
	)
	return scanCampaignConfigVersion(row)
}

func scanCampaignConfigVersion(scanner campaignScanner) (*service.CampaignConfigVersion, error) {
	var item service.CampaignConfigVersion
	var poolRate, rankRatio, contributionRatio string
	var weightsRaw string
	err := scanner.Scan(
		&item.ID, &item.CampaignID, &item.Version, &item.VersionScope, &item.EffectiveAt,
		&item.RechargeThresholdCents, &item.AllowAccumulatedRecharge, &poolRate, &rankRatio,
		&contributionRatio, &item.RankRewardCount, &weightsRaw, &item.MinPayoutAmountCents,
		&item.PayoutMethod, &item.PayoutChannel, &item.ChangeReason, &item.CreatedBy, &item.CreatedAt,
	)
	if err != nil {
		return nil, campaignRepoErr(err)
	}
	item.PoolInjectionRate, _ = decimal.NewFromString(poolRate)
	item.RankPoolRatio, _ = decimal.NewFromString(rankRatio)
	item.ContributionPoolRatio, _ = decimal.NewFromString(contributionRatio)
	_ = json.Unmarshal([]byte(weightsRaw), &item.RankWeights)
	return &item, nil
}

func scanCampaignInviteRecord(scanner campaignScanner) (*service.CampaignInviteRecord, error) {
	var item service.CampaignInviteRecord
	err := scanner.Scan(
		&item.ID, &item.CampaignID, &item.ConfigVersionID, &item.InviterUserID, &item.InviteeUserID,
		&item.InviteSource, &item.ThresholdSnapshotCents, &item.RegisteredAt, &item.QualifiedAt,
		&item.EffectiveRechargeAmountCents, &item.Status, &item.RiskLevel, &item.InvalidReason,
		&item.AuditStatus, &item.AuditBy, &item.AuditAt, &item.AuditNote,
	)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func scanCampaignInviteRecordWithUser(scanner campaignScanner) (*service.CampaignInviteRecord, error) {
	var item service.CampaignInviteRecord
	var email string
	if err := scanner.Scan(
		&item.ID, &item.CampaignID, &item.ConfigVersionID, &item.InviterUserID, &item.InviteeUserID,
		&item.InviteSource, &item.ThresholdSnapshotCents, &item.RegisteredAt, &item.QualifiedAt,
		&item.EffectiveRechargeAmountCents, &item.Status, &item.RiskLevel, &item.InvalidReason,
		&item.AuditStatus, &item.AuditBy, &item.AuditAt, &item.AuditNote,
		&email, &item.InviteeUsername,
	); err != nil {
		return nil, err
	}
	item.InviteeMaskedEmail = maskEmail(email)
	return &item, nil
}

func campaignLeaderboardSQL() string {
	return `
WITH ranked AS (
	SELECT
		cir.inviter_user_id AS user_id,
		COALESCE(u.email, '') AS email,
		COALESCE(u.username, '') AS username,
		COUNT(*) FILTER (WHERE cir.status = 'effective')::integer AS valid_invite_count,
		COALESCE(SUM(cir.effective_recharge_amount_cents) FILTER (WHERE cir.status = 'effective'), 0)::bigint AS recharge_amount,
		COALESCE(MAX(cir.qualified_at) FILTER (WHERE cir.status = 'effective'), MIN(cir.registered_at)) AS reached_count_at,
		MIN(cir.registered_at) AS joined_at
	FROM campaign_invite_records cir
	LEFT JOIN users u ON u.id = cir.inviter_user_id
	WHERE cir.campaign_id = $1
	GROUP BY cir.inviter_user_id, u.email, u.username
	HAVING COUNT(*) FILTER (WHERE cir.status = 'effective') > 0
)
SELECT
	ROW_NUMBER() OVER (ORDER BY valid_invite_count DESC, recharge_amount DESC, reached_count_at ASC, joined_at ASC)::integer AS rank,
	user_id, email, username, valid_invite_count, recharge_amount, reached_count_at, joined_at,
	0::bigint AS estimated_reward_cents, 0::bigint AS final_reward_cents
FROM ranked
ORDER BY rank`
}

func scanCampaignLeaderboardRows(rows *sql.Rows) ([]service.CampaignLeaderboardRow, error) {
	result := make([]service.CampaignLeaderboardRow, 0)
	for rows.Next() {
		var item service.CampaignLeaderboardRow
		var email string
		if err := rows.Scan(
			&item.Rank, &item.UserID, &email, &item.Username, &item.ValidInviteCount,
			&item.InviteeRechargeAmountCents, &item.ReachedCountAt, &item.JoinedAt,
			&item.EstimatedRewardCents, &item.FinalRewardCents,
		); err != nil {
			return nil, err
		}
		item.MaskedEmail = maskEmail(email)
		result = append(result, item)
	}
	return result, rows.Err()
}

func scanCampaignRewardResult(scanner campaignScanner) (*service.CampaignRewardResult, error) {
	var item service.CampaignRewardResult
	var rank sql.NullInt64
	var weight string
	err := scanner.Scan(
		&item.ID, &item.CampaignID, &item.ConfigVersionID, &item.CalculationBatchNo, &item.UserID, &rank,
		&item.RankRewardAmountCents, &weight, &item.ContributionRewardAmountCents, &item.GrossRewardAmountCents,
		&item.MinPayoutAmountSnapshotCents, &item.FinalPayoutAmountCents, &item.WithheldAmountCents,
		&item.WithheldReason, &item.RoundingResidualCents, &item.CalculationStatus, &item.CalculatedAt,
	)
	if err != nil {
		return nil, err
	}
	if rank.Valid {
		v := int(rank.Int64)
		item.Rank = &v
	}
	item.ContributionWeight, _ = decimal.NewFromString(weight)
	return &item, nil
}

func scanCampaignPayoutBatch(scanner campaignScanner) (*service.CampaignPayoutBatch, error) {
	var item service.CampaignPayoutBatch
	err := scanner.Scan(
		&item.ID, &item.CampaignID, &item.BatchNo, &item.Status, &item.OperatorID,
		&item.TotalUsers, &item.TotalAmountCents, &item.SuccessCount, &item.FailedCount,
		&item.StartedAt, &item.FinishedAt, &item.CreatedAt,
	)
	if err != nil {
		return nil, campaignRepoErr(err)
	}
	return &item, nil
}

func ensureCampaignParticipant(ctx context.Context, tx *sql.Tx, campaignID, userID int64, inviteCode string) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO campaign_participants (
	campaign_id, user_id, invite_code_snapshot, invite_link_snapshot, participant_status, joined_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, 'normal', NOW(), NOW(), NOW())
ON CONFLICT (campaign_id, user_id) DO NOTHING`,
		campaignID, userID, inviteCode, "/register?aff="+inviteCode)
	return err
}

func refreshCampaignParticipantStats(ctx context.Context, tx *sql.Tx, campaignID, userID int64) error {
	_, err := tx.ExecContext(ctx, `
UPDATE campaign_participants cp
SET valid_invite_count = COALESCE(s.valid_count, 0),
	pending_invite_count = COALESCE(s.pending_count, 0),
	invalid_invite_count = COALESCE(s.invalid_count, 0),
	invitee_recharge_amount_cents = COALESCE(s.recharge_amount, 0),
	updated_at = NOW()
FROM (
	SELECT
		COUNT(*) FILTER (WHERE status = 'effective')::integer AS valid_count,
		COUNT(*) FILTER (WHERE status IN ('registered', 'recharge_unqualified', 'pending_audit', 'risk_review'))::integer AS pending_count,
		COUNT(*) FILTER (WHERE status = 'invalid')::integer AS invalid_count,
		COALESCE(SUM(effective_recharge_amount_cents) FILTER (WHERE status = 'effective'), 0)::bigint AS recharge_amount
	FROM campaign_invite_records
	WHERE campaign_id = $1 AND inviter_user_id = $2
) s
WHERE cp.campaign_id = $1 AND cp.user_id = $2`, campaignID, userID)
	return err
}

func weightsJSON(weights []int64) string {
	b, _ := json.Marshal(weights)
	return string(b)
}

func nullableInt64(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func campaignRepoErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrCampaignNotFound
	}
	return err
}

func maskEmail(email string) string {
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
