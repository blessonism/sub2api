//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCurrentVisibleRateForPaymentPlan(t *testing.T) {
	visibleRate := 0.8
	g := &Group{RateMultiplier: 1.6, VisibleRateMultiplier: &visibleRate}
	require.Equal(t, 0.8, g.CurrentVisibleRate(timeRateAt(12, 0)))
	g.VisibleRateMultiplier = nil
	require.Equal(t, 1.6, g.CurrentVisibleRate(timeRateAt(12, 0)))
}
