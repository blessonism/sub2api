//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVisibleRateMultiplierForPaymentPlan(t *testing.T) {
	visibleRate := 0.8

	require.Equal(t, 0.8, visibleRateMultiplierForPaymentPlan(1.6, &visibleRate))
	require.Equal(t, 1.6, visibleRateMultiplierForPaymentPlan(1.6, nil))
}
