package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTokenAllocationDateRange_EndOfDayExclusiveDate(t *testing.T) {
	start := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 21, 15, 30, 0, 0, time.UTC)

	startDate, endDate, ok := tokenAllocationDateRange(start, end)

	require.True(t, ok)
	require.Equal(t, "2026-06-21", startDate)
	require.Equal(t, "2026-06-22", endDate)
}

func TestTokenAllocationDateRange_MidnightUsesSameExclusiveDate(t *testing.T) {
	start := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)

	startDate, endDate, ok := tokenAllocationDateRange(start, end)

	require.True(t, ok)
	require.Equal(t, "2026-06-21", startDate)
	require.Equal(t, "2026-06-22", endDate)
}

func TestValidateAdminUsageConsumptionCalibrationInput(t *testing.T) {
	tests := []struct {
		name    string
		input   AdminUsageConsumptionCalibrationInput
		wantErr bool
	}{
		{name: "valid target", input: AdminUsageConsumptionCalibrationInput{Mode: "target", Value: 10, StartDate: "2026-06-01", EndDate: "2026-06-02", Timezone: "Asia/Shanghai"}},
		{name: "negative target", input: AdminUsageConsumptionCalibrationInput{Mode: "target", Value: -1, StartDate: "2026-06-01", EndDate: "2026-06-02"}, wantErr: true},
		{name: "reversed range", input: AdminUsageConsumptionCalibrationInput{Mode: "delta", Value: 1, StartDate: "2026-06-02", EndDate: "2026-06-01"}, wantErr: true},
		{name: "invalid timezone", input: AdminUsageConsumptionCalibrationInput{Mode: "delta", Value: 1, StartDate: "2026-06-01", EndDate: "2026-06-02", Timezone: "Mars/Olympus"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAdminUsageConsumptionCalibrationInput(&tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
