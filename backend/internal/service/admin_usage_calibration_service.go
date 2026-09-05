package service

import (
	"context"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	AdminUsageCalibrationModeDelta  = "delta"
	AdminUsageCalibrationModeTarget = "target"
)

var (
	ErrAdminUsageCalibrationInvalidInput = infraerrors.BadRequest("ADMIN_USAGE_CALIBRATION_INVALID_INPUT", "invalid calibration input")
	ErrAdminUsageCalibrationNotFound    = infraerrors.NotFound("ADMIN_USAGE_CALIBRATION_NOT_FOUND", "usage calibration not found")
	ErrAdminUsageCalibrationRevoked     = infraerrors.Conflict("ADMIN_USAGE_CALIBRATION_ALREADY_REVOKED", "usage calibration has already been revoked")
)

type AdminUsageTokenCalibrationInput struct {
	Mode      string `json:"mode"`
	Value     int64  `json:"value"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Timezone  string `json:"timezone"`
}

type AdminUsageBalanceCalibrationInput struct {
	Mode  string  `json:"mode"`
	Value float64 `json:"value"`
}

type AdminUsageConsumptionCalibrationInput struct {
	Mode      string  `json:"mode"`
	Value     float64 `json:"value"`
	StartDate string  `json:"start_date"`
	EndDate   string  `json:"end_date"`
	Timezone  string  `json:"timezone"`
}

type AdminUsageCalibrationCreateInput struct {
	TargetUserID int64                                  `json:"target_user_id"`
	AdminUserID  int64                                  `json:"admin_user_id"`
	Reason       string                                 `json:"reason"`
	Token        *AdminUsageTokenCalibrationInput       `json:"token,omitempty"`
	Balance      *AdminUsageBalanceCalibrationInput     `json:"balance,omitempty"`
	Consumption  *AdminUsageConsumptionCalibrationInput `json:"consumption,omitempty"`
}

type AdminUsageCalibrationDailyAllocation struct {
	ID            int64     `json:"id"`
	CalibrationID int64     `json:"calibration_id"`
	TargetUserID  int64     `json:"target_user_id"`
	Date          string    `json:"date"`
	OriginalToken int64     `json:"original_tokens"`
	TokenDelta    int64     `json:"token_delta"`
	BalanceDelta  *float64  `json:"balance_delta,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type AdminUsageCalibration struct {
	ID                        int64                                  `json:"id"`
	TargetUserID              int64                                  `json:"target_user_id"`
	AdminUserID               int64                                  `json:"admin_user_id"`
	Reason                    string                                 `json:"reason"`
	TokenMode                 *string                                `json:"token_mode,omitempty"`
	TokenInputValue           *int64                                 `json:"token_input_value,omitempty"`
	TokenBeforeValue          *int64                                 `json:"token_before_value,omitempty"`
	TokenAfterValue           *int64                                 `json:"token_after_value,omitempty"`
	TokenDelta                *int64                                 `json:"token_delta,omitempty"`
	TokenCalculationStartDate *string                                `json:"token_calculation_start_date,omitempty"`
	TokenCalculationEndDate   *string                                `json:"token_calculation_end_date,omitempty"`
	TokenCalculationTimezone  *string                                `json:"token_calculation_timezone,omitempty"`
	BalanceMode               *string                                `json:"balance_mode,omitempty"`
	BalanceInputValue         *float64                               `json:"balance_input_value,omitempty"`
	BalanceBeforeValue        *float64                               `json:"balance_before_value,omitempty"`
	BalanceAfterValue         *float64                               `json:"balance_after_value,omitempty"`
	BalanceDelta              *float64                               `json:"balance_delta,omitempty"`
	ConsumptionMode           *string                                `json:"consumption_mode,omitempty"`
	ConsumptionInputValue     *float64                               `json:"consumption_input_value,omitempty"`
	ConsumptionBeforeValue    *float64                               `json:"consumption_before_value,omitempty"`
	ConsumptionAfterValue     *float64                               `json:"consumption_after_value,omitempty"`
	ConsumptionDelta          *float64                               `json:"consumption_delta,omitempty"`
	ConsumptionStartDate      *string                                `json:"consumption_start_date,omitempty"`
	ConsumptionEndDate        *string                                `json:"consumption_end_date,omitempty"`
	ConsumptionTimezone       *string                                `json:"consumption_timezone,omitempty"`
	CreatedAt                 time.Time                              `json:"created_at"`
	RevokedAt                 *time.Time                             `json:"revoked_at,omitempty"`
	RevokedBy                 *int64                                 `json:"revoked_by,omitempty"`
	Allocations               []AdminUsageCalibrationDailyAllocation `json:"allocations,omitempty"`
}

type AdminUsageCalibrationListFilters struct {
	TargetUserID int64
}

type AdminUsageCalibrationRepository interface {
	CreateAdminUsageCalibration(ctx context.Context, input AdminUsageCalibrationCreateInput) (*AdminUsageCalibration, error)
	RevokeAdminUsageCalibration(ctx context.Context, calibrationID, adminUserID int64) (*AdminUsageCalibration, error)
	ListAdminUsageCalibrations(ctx context.Context, filters AdminUsageCalibrationListFilters, params pagination.PaginationParams) ([]AdminUsageCalibration, *pagination.PaginationResult, error)
	SumTokenAllocations(ctx context.Context, userID int64, startDate, endDateExclusive string) (int64, error)
	SumAllTokenAllocations(ctx context.Context, startDate, endDateExclusive string) (int64, error)
	SumTokenAllocationsByDate(ctx context.Context, userID int64, startDate, endDateExclusive string) (map[string]int64, error)
	SumBalanceSpent(ctx context.Context, userID int64, startTime, endTime time.Time) (float64, error)
	SumBalanceSpentByUsers(ctx context.Context, userIDs []int64, startTime, endTime time.Time) (map[int64]float64, error)
}

type AdminUsageCalibrationService struct {
	repo                 AdminUsageCalibrationRepository
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
}

func NewAdminUsageCalibrationService(repo AdminUsageCalibrationRepository, authCacheInvalidator APIKeyAuthCacheInvalidator, billingCacheService *BillingCacheService) *AdminUsageCalibrationService {
	return &AdminUsageCalibrationService{
		repo:                 repo,
		authCacheInvalidator: authCacheInvalidator,
		billingCacheService:  billingCacheService,
	}
}

func (s *AdminUsageCalibrationService) Create(ctx context.Context, input AdminUsageCalibrationCreateInput) (*AdminUsageCalibration, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.New(503, "ADMIN_USAGE_CALIBRATION_UNAVAILABLE", "usage calibration service unavailable")
	}
	if err := validateAdminUsageCalibrationInput(&input); err != nil {
		return nil, err
	}

	record, err := s.repo.CreateAdminUsageCalibration(ctx, input)
	if err != nil {
		return nil, err
	}
	if record != nil && record.BalanceDelta != nil && *record.BalanceDelta != 0 {
		s.invalidateBalanceCaches(ctx, record.TargetUserID)
	}
	return record, nil
}

func (s *AdminUsageCalibrationService) List(ctx context.Context, filters AdminUsageCalibrationListFilters, params pagination.PaginationParams) ([]AdminUsageCalibration, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, infraerrors.New(503, "ADMIN_USAGE_CALIBRATION_UNAVAILABLE", "usage calibration service unavailable")
	}
	if filters.TargetUserID < 0 {
		return nil, nil, ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "user_id"})
	}
	return s.repo.ListAdminUsageCalibrations(ctx, filters, params)
}

func (s *AdminUsageCalibrationService) Revoke(ctx context.Context, calibrationID, adminUserID int64) (*AdminUsageCalibration, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.New(503, "ADMIN_USAGE_CALIBRATION_UNAVAILABLE", "usage calibration service unavailable")
	}
	if calibrationID <= 0 || adminUserID <= 0 {
		return nil, ErrAdminUsageCalibrationInvalidInput
	}
	record, err := s.repo.RevokeAdminUsageCalibration(ctx, calibrationID, adminUserID)
	if err != nil {
		return nil, err
	}
	if record != nil && record.BalanceDelta != nil && *record.BalanceDelta != 0 {
		s.invalidateBalanceCaches(ctx, record.TargetUserID)
	}
	return record, nil
}

func (s *AdminUsageCalibrationService) invalidateBalanceCaches(ctx context.Context, userID int64) {
	if userID <= 0 {
		return
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.billingCacheService.InvalidateUserBalance(cacheCtx, userID); err != nil {
				logger.LegacyPrintf("service.admin_usage_calibration", "invalidate user balance cache failed: user_id=%d err=%v", userID, err)
			}
		}()
	}
}

func validateAdminUsageCalibrationInput(input *AdminUsageCalibrationCreateInput) error {
	if input == nil {
		return ErrAdminUsageCalibrationInvalidInput
	}
	if input.TargetUserID <= 0 {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "target_user_id"})
	}
	if input.AdminUserID <= 0 {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "admin_user_id"})
	}
	input.Reason = ""
	if input.Token == nil && input.Balance == nil && input.Consumption == nil {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "calibration"})
	}
	if input.Token != nil {
		if err := validateAdminUsageTokenCalibrationInput(input.Token); err != nil {
			return err
		}
	}
	if input.Balance != nil {
		if err := validateAdminUsageBalanceCalibrationInput(input.Balance); err != nil {
			return err
		}
	}
	if input.Consumption != nil {
		if input.Balance != nil {
			return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "balance"})
		}
		if err := validateAdminUsageConsumptionCalibrationInput(input.Consumption); err != nil {
			return err
		}
	}
	return nil
}

func validateAdminUsageTokenCalibrationInput(input *AdminUsageTokenCalibrationInput) error {
	input.Mode = strings.TrimSpace(input.Mode)
	if input.Mode != AdminUsageCalibrationModeDelta && input.Mode != AdminUsageCalibrationModeTarget {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "token.mode"})
	}
	input.StartDate = strings.TrimSpace(input.StartDate)
	input.EndDate = strings.TrimSpace(input.EndDate)
	start, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "token.start_date"})
	}
	end, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "token.end_date"})
	}
	if end.Before(start) {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "token.date_range"})
	}
	if input.Mode == AdminUsageCalibrationModeTarget && input.Value < 0 {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "token.value"})
	}
	input.Timezone = strings.TrimSpace(input.Timezone)
	if input.Timezone != "" {
		if _, err := time.LoadLocation(input.Timezone); err != nil {
			return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "token.timezone"})
		}
	}
	return nil
}

func validateAdminUsageBalanceCalibrationInput(input *AdminUsageBalanceCalibrationInput) error {
	input.Mode = strings.TrimSpace(input.Mode)
	if input.Mode != AdminUsageCalibrationModeDelta && input.Mode != AdminUsageCalibrationModeTarget {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "balance.mode"})
	}
	if math.IsNaN(input.Value) || math.IsInf(input.Value, 0) {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "balance.value"})
	}
	if input.Mode == AdminUsageCalibrationModeTarget && input.Value < 0 {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "balance.value"})
	}
	return nil
}

func validateAdminUsageConsumptionCalibrationInput(input *AdminUsageConsumptionCalibrationInput) error {
	input.Mode = strings.TrimSpace(input.Mode)
	if input.Mode != AdminUsageCalibrationModeDelta && input.Mode != AdminUsageCalibrationModeTarget {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "consumption.mode"})
	}
	if math.IsNaN(input.Value) || math.IsInf(input.Value, 0) || input.Mode == AdminUsageCalibrationModeTarget && input.Value < 0 {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "consumption.value"})
	}
	input.StartDate = strings.TrimSpace(input.StartDate)
	input.EndDate = strings.TrimSpace(input.EndDate)
	start, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "consumption.start_date"})
	}
	end, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil || end.Before(start) {
		return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "consumption.date_range"})
	}
	input.Timezone = strings.TrimSpace(input.Timezone)
	if input.Timezone != "" {
		if _, err := time.LoadLocation(input.Timezone); err != nil {
			return ErrAdminUsageCalibrationInvalidInput.WithMetadata(map[string]string{"field": "consumption.timezone"})
		}
	}
	return nil
}

func tokenAllocationDateRange(startTime, endTime time.Time) (string, string, bool) {
	if startTime.IsZero() && endTime.IsZero() {
		return "", "", true
	}
	if !endTime.IsZero() && !startTime.IsZero() && !endTime.After(startTime) {
		return "", "", false
	}
	startDate := ""
	if !startTime.IsZero() {
		startDate = startTime.Format("2006-01-02")
	}
	endDateExclusive := ""
	if !endTime.IsZero() {
		endDateExclusive = calibrationExclusiveDateForTime(endTime)
	}
	return startDate, endDateExclusive, true
}

func calibrationExclusiveDateForTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
		return t.Format("2006-01-02")
	}
	return t.AddDate(0, 0, 1).Format("2006-01-02")
}
