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
