package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type lotteryCampaignRepository struct {
	db *sql.DB
}

func NewLotteryCampaignRepository(db *sql.DB) service.LotteryCampaignRepository {
	return &lotteryCampaignRepository{db: db}
}

func (r *lotteryCampaignRepository) ListLotteryCampaigns(ctx context.Context, page, pageSize int) ([]service.LotteryCampaign, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM lottery_campaigns`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, description, rules_text, status, participation_mode, draw_schedule_type, prize_mode,
	entry_mode, threshold_tokens, entry_step_tokens, max_entries_per_user, start_at, end_at, draw_at,
	daily_draw_time, created_by, updated_by, created_at, updated_at
FROM lottery_campaigns
ORDER BY created_at DESC, id DESC
LIMIT $1 OFFSET $2`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]service.LotteryCampaign, 0)
	for rows.Next() {
		item, err := scanLotteryCampaign(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := r.attachPrizeTiers(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *lotteryCampaignRepository) CreateLotteryCampaign(ctx context.Context, input service.LotteryCampaignInput) (*service.LotteryCampaign, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var id int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO lottery_campaigns (
	name, description, rules_text, status, participation_mode, draw_schedule_type, prize_mode, entry_mode,
	threshold_tokens, entry_step_tokens, max_entries_per_user, start_at, end_at, draw_at, daily_draw_time,
	created_by, updated_by, created_at, updated_at
) VALUES ($1, $2, $3, 'draft', $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $15, NOW(), NOW())
RETURNING id`,
		input.Name, input.Description, input.RulesText, input.ParticipationMode, input.DrawScheduleType,
		input.PrizeMode, input.EntryMode, input.ThresholdTokens, input.EntryStepTokens, input.MaxEntriesPerUser,
		input.StartAt, input.EndAt, nullableTime(input.DrawAt), input.DailyDrawTime, nullableInt64(input.OperatorID),
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	if err := replaceLotteryPrizeTiers(ctx, tx, id, input.PrizeTiers); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetLotteryCampaign(ctx, id)
}

func (r *lotteryCampaignRepository) UpdateLotteryCampaign(ctx context.Context, id int64, input service.LotteryCampaignInput) (*service.LotteryCampaign, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `
UPDATE lottery_campaigns
SET name = $2, description = $3, rules_text = $4, participation_mode = $5, draw_schedule_type = $6,
	prize_mode = $7, entry_mode = $8, threshold_tokens = $9, entry_step_tokens = $10,
	max_entries_per_user = $11, start_at = $12, end_at = $13, draw_at = $14, daily_draw_time = $15,
	updated_by = $16, updated_at = NOW()
WHERE id = $1 AND status = 'draft'`,
		id, input.Name, input.Description, input.RulesText, input.ParticipationMode, input.DrawScheduleType,
		input.PrizeMode, input.EntryMode, input.ThresholdTokens, input.EntryStepTokens, input.MaxEntriesPerUser,
		input.StartAt, input.EndAt, nullableTime(input.DrawAt), input.DailyDrawTime, nullableInt64(input.OperatorID),
	)
	if err != nil {
		return nil, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, service.ErrLotteryCampaignNotFound
	}
	if err := replaceLotteryPrizeTiers(ctx, tx, id, input.PrizeTiers); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetLotteryCampaign(ctx, id)
}

func (r *lotteryCampaignRepository) GetLotteryCampaign(ctx context.Context, id int64) (*service.LotteryCampaign, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, name, description, rules_text, status, participation_mode, draw_schedule_type, prize_mode,
	entry_mode, threshold_tokens, entry_step_tokens, max_entries_per_user, start_at, end_at, draw_at,
	daily_draw_time, created_by, updated_by, created_at, updated_at
FROM lottery_campaigns
WHERE id = $1`, id)
	item, err := scanLotteryCampaign(row)
	if err != nil {
		return nil, lotteryRepoErr(err)
	}
	tiers, err := r.listPrizeTiers(ctx, id)
	if err != nil {
		return nil, err
	}
	item.PrizeTiers = tiers
	return item, nil
}

func (r *lotteryCampaignRepository) GetActiveLotteryCampaign(ctx context.Context, now time.Time) (*service.LotteryCampaign, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, name, description, rules_text, status, participation_mode, draw_schedule_type, prize_mode,
	entry_mode, threshold_tokens, entry_step_tokens, max_entries_per_user, start_at, end_at, draw_at,
	daily_draw_time, created_by, updated_by, created_at, updated_at
FROM lottery_campaigns
WHERE status = 'published'
  AND start_at <= $1
  AND end_at >= $1
ORDER BY start_at ASC, id ASC
LIMIT 1`, now)
	item, err := scanLotteryCampaign(row)
	if err != nil {
		return nil, lotteryRepoErr(err)
	}
	tiers, err := r.listPrizeTiers(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	item.PrizeTiers = tiers
	return item, nil
}

func (r *lotteryCampaignRepository) UpdateLotteryCampaignStatus(ctx context.Context, id int64, status string, operatorID *int64) (*service.LotteryCampaign, error) {
	res, err := r.db.ExecContext(ctx, `
UPDATE lottery_campaigns
SET status = $2, updated_by = $3, updated_at = NOW()
WHERE id = $1`, id, status, nullableInt64(operatorID))
	if err != nil {
		return nil, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, service.ErrLotteryCampaignNotFound
	}
	return r.GetLotteryCampaign(ctx, id)
}

func (r *lotteryCampaignRepository) ListPublishedLotteryCampaigns(ctx context.Context) ([]service.LotteryCampaign, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, description, rules_text, status, participation_mode, draw_schedule_type, prize_mode,
	entry_mode, threshold_tokens, entry_step_tokens, max_entries_per_user, start_at, end_at, draw_at,
	daily_draw_time, created_by, updated_by, created_at, updated_at
FROM lottery_campaigns
WHERE status = 'published'
ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]service.LotteryCampaign, 0)
	for rows.Next() {
		item, err := scanLotteryCampaign(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.attachPrizeTiers(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *lotteryCampaignRepository) GetLotteryUserTokens(ctx context.Context, userID int64, startAt, endAt time.Time) (int64, error) {
	var tokens int64
	err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0)
FROM usage_logs
WHERE user_id = $1 AND created_at >= $2 AND created_at < $3 AND actual_cost > 0`, userID, startAt, endAt).Scan(&tokens)
	return tokens, err
}

func (r *lotteryCampaignRepository) ListLotteryQualifiedUsage(ctx context.Context, startAt, endAt time.Time, minTokens int64) ([]service.LotteryQualifiedUsage, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT user_id, COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) AS tokens
FROM usage_logs
WHERE created_at >= $1 AND created_at < $2 AND actual_cost > 0
GROUP BY user_id
HAVING COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) >= $3
ORDER BY user_id ASC`, startAt, endAt, minTokens)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]service.LotteryQualifiedUsage, 0)
	for rows.Next() {
		var item service.LotteryQualifiedUsage
		if err := rows.Scan(&item.UserID, &item.Tokens); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *lotteryCampaignRepository) UpsertLotteryEntry(ctx context.Context, campaign service.LotteryCampaign, userID int64, entryDate time.Time, tokens int64, entryCount int, enrolled bool) (*service.LotteryEntry, error) {
	status := service.LotteryEntryEligible
	if enrolled {
		status = service.LotteryEntryEnrolled
	}
	row := r.db.QueryRowContext(ctx, `
INSERT INTO lottery_entries (campaign_id, user_id, entry_date, tokens, entry_count, status, enrolled_at, created_at, updated_at)
VALUES ($1, $2, $3::date, $4, $5, $6, CASE WHEN $6 = 'enrolled' THEN NOW() ELSE NULL END, NOW(), NOW())
ON CONFLICT (campaign_id, user_id, entry_date) DO UPDATE
SET tokens = EXCLUDED.tokens,
	entry_count = EXCLUDED.entry_count,
	status = CASE WHEN lottery_entries.status = 'enrolled' THEN 'enrolled' ELSE EXCLUDED.status END,
	enrolled_at = CASE
		WHEN lottery_entries.enrolled_at IS NOT NULL THEN lottery_entries.enrolled_at
		WHEN EXCLUDED.status = 'enrolled' THEN NOW()
		ELSE NULL
	END,
	updated_at = NOW()
RETURNING id, campaign_id, user_id, entry_date, tokens, entry_count, status, enrolled_at, created_at, updated_at`,
		campaign.ID, userID, entryDate, tokens, entryCount, status)
	return scanLotteryEntry(row)
}

func (r *lotteryCampaignRepository) GetLotteryEntry(ctx context.Context, campaignID, userID int64, entryDate time.Time) (*service.LotteryEntry, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, campaign_id, user_id, entry_date, tokens, entry_count, status, enrolled_at, created_at, updated_at
FROM lottery_entries
WHERE campaign_id = $1 AND user_id = $2 AND entry_date = $3::date`, campaignID, userID, entryDate)
	entry, err := scanLotteryEntry(row)
	if err != nil {
		return nil, lotteryRepoErr(err)
	}
	return entry, nil
}

func (r *lotteryCampaignRepository) ListLotteryDrawCandidates(ctx context.Context, campaignID int64, entryDate time.Time) ([]service.LotteryDrawCandidate, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT user_id, entry_date, tokens, entry_count
FROM lottery_entries
WHERE campaign_id = $1 AND entry_date = $2::date AND status = 'enrolled'
ORDER BY user_id ASC`, campaignID, entryDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]service.LotteryDrawCandidate, 0)
	for rows.Next() {
		var item service.LotteryDrawCandidate
		if err := rows.Scan(&item.UserID, &item.EntryDate, &item.Tokens, &item.EntryCount); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *lotteryCampaignRepository) GetLotteryDrawBatch(ctx context.Context, campaignID int64, drawDate time.Time) (*service.LotteryDrawBatch, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, campaign_id, draw_date, scheduled_draw_at, batch_no, status, trigger_type, operator_id,
	total_entries, total_winners, error_message, created_at, drawn_at, finished_at
FROM lottery_draw_batches
WHERE campaign_id = $1 AND draw_date = $2::date`, campaignID, drawDate)
	item, err := scanLotteryDrawBatch(row)
	if err != nil {
		return nil, lotteryRepoErr(err)
	}
	return item, nil
}

func (r *lotteryCampaignRepository) CreateLotteryDrawBatch(ctx context.Context, campaignID int64, drawDate, scheduledDrawAt time.Time, triggerType string, operatorID *int64) (*service.LotteryDrawBatch, error) {
	batchNo := fmt.Sprintf("lottery-%d-%s-%d", campaignID, drawDate.Format("20060102"), time.Now().UnixNano())
	row := r.db.QueryRowContext(ctx, `
INSERT INTO lottery_draw_batches (campaign_id, draw_date, scheduled_draw_at, batch_no, status, trigger_type, operator_id, created_at)
VALUES ($1, $2::date, $3, $4, 'processing', $5, $6, NOW())
ON CONFLICT (campaign_id, draw_date) DO UPDATE SET campaign_id = lottery_draw_batches.campaign_id
RETURNING id, campaign_id, draw_date, scheduled_draw_at, batch_no, status, trigger_type, operator_id,
	total_entries, total_winners, error_message, created_at, drawn_at, finished_at`,
		campaignID, drawDate, scheduledDrawAt, batchNo, triggerType, nullableInt64(operatorID))
	return scanLotteryDrawBatch(row)
}

func (r *lotteryCampaignRepository) CreateLotteryWinners(ctx context.Context, batch service.LotteryDrawBatch, campaign service.LotteryCampaign, winners []service.LotteryWinner) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, winner := range winners {
		_, err := tx.ExecContext(ctx, `
INSERT INTO lottery_winners (
	batch_id, campaign_id, user_id, prize_tier_id, entry_date, reward_amount_cents, status, idempotency_key, created_at
) VALUES ($1, $2, $3, $4, $5::date, $6, 'pending', $7, NOW())
ON CONFLICT (idempotency_key) DO NOTHING`,
			batch.ID, campaign.ID, winner.UserID, winner.PrizeTierID, winner.EntryDate, winner.RewardAmountCents, winner.IdempotencyKey)
		if err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `
UPDATE lottery_draw_batches
SET total_entries = (SELECT COUNT(*) FROM lottery_entries WHERE campaign_id = $1 AND entry_date = $2::date AND status = 'enrolled'),
	total_winners = (SELECT COUNT(*) FROM lottery_winners WHERE batch_id = $3),
	drawn_at = NOW()
WHERE id = $3`, campaign.ID, batch.DrawDate, batch.ID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *lotteryCampaignRepository) GrantLotteryWinnerBalance(ctx context.Context, winnerID int64, notes string) (*service.LotteryBalanceGrantResult, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		userID                int64
		rewardAmountCents     int64
		status                string
		currentBalance        float64
		balanceBeforeSnapshot sql.NullFloat64
		balanceAfterSnapshot  sql.NullFloat64
	)
	err = tx.QueryRowContext(ctx, `
SELECT w.user_id, w.reward_amount_cents, w.status,
	w.balance_before_snapshot, w.balance_after_snapshot, u.balance
FROM lottery_winners w
JOIN users u ON u.id = w.user_id
WHERE w.id = $1
FOR UPDATE OF w, u`, winnerID).Scan(
		&userID, &rewardAmountCents, &status, &balanceBeforeSnapshot, &balanceAfterSnapshot, &currentBalance,
	)
	if err != nil {
		return nil, false, lotteryRepoErr(err)
	}
	if status == "success" {
		return &service.LotteryBalanceGrantResult{
			WinnerID:      winnerID,
			UserID:        userID,
			BalanceBefore: balanceBeforeSnapshot.Float64,
			BalanceAfter:  balanceAfterSnapshot.Float64,
		}, false, nil
	}

	amount := float64(rewardAmountCents) / 100
	before := currentBalance
	after := before + amount
	code, err := service.GenerateRedeemCode()
	if err != nil {
		return nil, false, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE users
SET balance = $2, updated_at = NOW()
WHERE id = $1`, userID, after); err != nil {
		return nil, false, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO redeem_codes (code, type, value, status, used_by, used_at, notes, created_at)
VALUES ($1, $2, $3, $4, $5, NOW(), $6, NOW())`,
		code, service.AdjustmentTypeAdminBalance, amount, service.StatusUsed, userID, notes,
	); err != nil {
		return nil, false, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE lottery_winners
SET status = 'success', balance_before_snapshot = $2, balance_after_snapshot = $3,
	error_message = '', processed_at = NOW()
WHERE id = $1`, winnerID, before, after); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return &service.LotteryBalanceGrantResult{
		WinnerID:      winnerID,
		UserID:        userID,
		BalanceBefore: before,
		BalanceAfter:  after,
	}, true, nil
}

func (r *lotteryCampaignRepository) MarkLotteryWinnerSuccess(ctx context.Context, winnerID int64, before, after float64) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE lottery_winners
SET status = 'success', balance_before_snapshot = $2, balance_after_snapshot = $3,
	error_message = '', processed_at = NOW()
WHERE id = $1 AND status <> 'success'`, winnerID, before, after)
	return err
}

func (r *lotteryCampaignRepository) MarkLotteryWinnerFailed(ctx context.Context, winnerID int64, message string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE lottery_winners
SET status = 'failed', error_message = $2, processed_at = NOW()
WHERE id = $1 AND status <> 'success'`, winnerID, message)
	return err
}

func (r *lotteryCampaignRepository) UpdateLotteryDrawBatchSummary(ctx context.Context, batchID int64) (*service.LotteryDrawBatch, error) {
	row := r.db.QueryRowContext(ctx, `
WITH summary AS (
	SELECT COUNT(*) AS total_winners,
		COUNT(*) FILTER (WHERE status = 'success') AS success_count,
		COUNT(*) FILTER (WHERE status = 'failed') AS failed_count
	FROM lottery_winners
	WHERE batch_id = $1
)
UPDATE lottery_draw_batches b
SET total_winners = summary.total_winners,
	status = CASE
		WHEN summary.total_winners = 0 THEN 'success'
		WHEN summary.failed_count = 0 THEN 'success'
		WHEN summary.success_count > 0 THEN 'partial_success'
		ELSE 'failed'
	END,
	finished_at = NOW()
FROM summary
WHERE b.id = $1
RETURNING b.id, b.campaign_id, b.draw_date, b.scheduled_draw_at, b.batch_no, b.status, b.trigger_type, b.operator_id,
	b.total_entries, b.total_winners, b.error_message, b.created_at, b.drawn_at, b.finished_at`, batchID)
	item, err := scanLotteryDrawBatch(row)
	if err != nil {
		return nil, lotteryRepoErr(err)
	}
	return item, nil
}

func (r *lotteryCampaignRepository) ListLotteryDrawBatches(ctx context.Context, campaignID int64, page, pageSize int) ([]service.LotteryDrawBatch, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM lottery_draw_batches WHERE campaign_id = $1`, campaignID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, campaign_id, draw_date, scheduled_draw_at, batch_no, status, trigger_type, operator_id,
	total_entries, total_winners, error_message, created_at, drawn_at, finished_at
FROM lottery_draw_batches
WHERE campaign_id = $1
ORDER BY draw_date DESC, id DESC
LIMIT $2 OFFSET $3`, campaignID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]service.LotteryDrawBatch, 0)
	for rows.Next() {
		item, err := scanLotteryDrawBatch(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *item)
	}
	return out, total, rows.Err()
}

func (r *lotteryCampaignRepository) ListLotteryWinners(ctx context.Context, campaignID int64, batchID *int64, userID *int64) ([]service.LotteryWinner, error) {
	args := []any{campaignID}
	query := `
SELECT w.id, w.batch_id, w.campaign_id, w.user_id, w.prize_tier_id, COALESCE(p.tier_name, '') AS prize_name,
	w.entry_date, w.reward_amount_cents, w.status, w.balance_before_snapshot, w.balance_after_snapshot,
	w.idempotency_key, w.error_message, w.created_at, w.processed_at
FROM lottery_winners w
LEFT JOIN lottery_prize_tiers p ON p.id = w.prize_tier_id
WHERE w.campaign_id = $1`
	if batchID != nil {
		args = append(args, *batchID)
		query += fmt.Sprintf(" AND w.batch_id = $%d", len(args))
	}
	if userID != nil {
		args = append(args, *userID)
		query += fmt.Sprintf(" AND w.user_id = $%d", len(args))
	}
	query += " ORDER BY w.created_at DESC, w.id DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]service.LotteryWinner, 0)
	for rows.Next() {
		item, err := scanLotteryWinner(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func replaceLotteryPrizeTiers(ctx context.Context, tx *sql.Tx, campaignID int64, tiers []service.LotteryPrizeTierInput) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM lottery_prize_tiers WHERE campaign_id = $1`, campaignID); err != nil {
		return err
	}
	for i, tier := range tiers {
		sortOrder := tier.SortOrder
		if sortOrder == 0 {
			sortOrder = i + 1
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO lottery_prize_tiers (campaign_id, tier_name, winner_count, reward_amount_cents, sort_order, created_at)
VALUES ($1, $2, $3, $4, $5, NOW())`, campaignID, tier.TierName, tier.WinnerCount, tier.RewardAmountCents, sortOrder); err != nil {
			return err
		}
	}
	return nil
}

func (r *lotteryCampaignRepository) attachPrizeTiers(ctx context.Context, campaigns []service.LotteryCampaign) error {
	for i := range campaigns {
		tiers, err := r.listPrizeTiers(ctx, campaigns[i].ID)
		if err != nil {
			return err
		}
		campaigns[i].PrizeTiers = tiers
	}
	return nil
}

func (r *lotteryCampaignRepository) listPrizeTiers(ctx context.Context, campaignID int64) ([]service.LotteryPrizeTier, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, campaign_id, tier_name, winner_count, reward_amount_cents, sort_order, created_at
FROM lottery_prize_tiers
WHERE campaign_id = $1
ORDER BY sort_order ASC, id ASC`, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]service.LotteryPrizeTier, 0)
	for rows.Next() {
		var item service.LotteryPrizeTier
		if err := rows.Scan(&item.ID, &item.CampaignID, &item.TierName, &item.WinnerCount, &item.RewardAmountCents, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

type lotteryScanner interface {
	Scan(dest ...any) error
}

func scanLotteryCampaign(scanner lotteryScanner) (*service.LotteryCampaign, error) {
	var item service.LotteryCampaign
	var drawAt sql.NullTime
	var createdBy, updatedBy sql.NullInt64
	if err := scanner.Scan(
		&item.ID, &item.Name, &item.Description, &item.RulesText, &item.Status, &item.ParticipationMode,
		&item.DrawScheduleType, &item.PrizeMode, &item.EntryMode, &item.ThresholdTokens, &item.EntryStepTokens,
		&item.MaxEntriesPerUser, &item.StartAt, &item.EndAt, &drawAt, &item.DailyDrawTime,
		&createdBy, &updatedBy, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if drawAt.Valid {
		item.DrawAt = &drawAt.Time
	}
	item.CreatedBy = lotteryNullInt64Ptr(createdBy)
	item.UpdatedBy = lotteryNullInt64Ptr(updatedBy)
	return &item, nil
}

func scanLotteryEntry(scanner lotteryScanner) (*service.LotteryEntry, error) {
	var item service.LotteryEntry
	var enrolledAt sql.NullTime
	if err := scanner.Scan(&item.ID, &item.CampaignID, &item.UserID, &item.EntryDate, &item.Tokens, &item.EntryCount, &item.Status, &enrolledAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	if enrolledAt.Valid {
		item.EnrolledAt = &enrolledAt.Time
	}
	return &item, nil
}

func scanLotteryDrawBatch(scanner lotteryScanner) (*service.LotteryDrawBatch, error) {
	var item service.LotteryDrawBatch
	var operatorID sql.NullInt64
	var drawnAt, finishedAt sql.NullTime
	if err := scanner.Scan(&item.ID, &item.CampaignID, &item.DrawDate, &item.ScheduledDrawAt, &item.BatchNo, &item.Status, &item.TriggerType, &operatorID, &item.TotalEntries, &item.TotalWinners, &item.ErrorMessage, &item.CreatedAt, &drawnAt, &finishedAt); err != nil {
		return nil, err
	}
	item.OperatorID = lotteryNullInt64Ptr(operatorID)
	if drawnAt.Valid {
		item.DrawnAt = &drawnAt.Time
	}
	if finishedAt.Valid {
		item.FinishedAt = &finishedAt.Time
	}
	return &item, nil
}

func scanLotteryWinner(scanner lotteryScanner) (*service.LotteryWinner, error) {
	var item service.LotteryWinner
	var before, after sql.NullFloat64
	var processedAt sql.NullTime
	if err := scanner.Scan(&item.ID, &item.BatchID, &item.CampaignID, &item.UserID, &item.PrizeTierID, &item.PrizeName, &item.EntryDate, &item.RewardAmountCents, &item.Status, &before, &after, &item.IdempotencyKey, &item.ErrorMessage, &item.CreatedAt, &processedAt); err != nil {
		return nil, err
	}
	if before.Valid {
		item.BalanceBeforeSnapshot = &before.Float64
	}
	if after.Valid {
		item.BalanceAfterSnapshot = &after.Float64
	}
	if processedAt.Valid {
		item.ProcessedAt = &processedAt.Time
	}
	return &item, nil
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func lotteryNullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	out := v.Int64
	return &out
}

func lotteryRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrLotteryCampaignNotFound
	}
	return err
}
