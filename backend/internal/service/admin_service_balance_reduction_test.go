//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type balanceReductionRepoStub struct {
	*userRepoStub
	previewFactor string
	reduceFactor  string
	operationID   string
}

func (s *balanceReductionRepoStub) PreviewAllUserBalanceReduction(_ context.Context, factor string) (*AdminBalanceReductionSummary, error) {
	s.previewFactor = factor
	return &AdminBalanceReductionSummary{Factor: factor, UserCount: 3}, nil
}

func (s *balanceReductionRepoStub) ReduceAllUserBalances(_ context.Context, factor, operationID, _ string) (*AdminBalanceReductionSummary, error) {
	s.reduceFactor = factor
	s.operationID = operationID
	return &AdminBalanceReductionSummary{OperationID: operationID, Factor: factor, UserIDs: []int64{2, 3}}, nil
}

func TestAdminServiceAllUserBalanceReductionNormalizesFactor(t *testing.T) {
	repo := &balanceReductionRepoStub{userRepoStub: &userRepoStub{}}
	svc := &adminServiceImpl{userRepo: repo}

	preview, err := svc.PreviewAllUserBalanceReduction(context.Background(), " 2.50000000 ")
	require.NoError(t, err)
	require.Equal(t, "2.5", preview.Factor)
	require.Equal(t, "2.5", repo.previewFactor)

	result, err := svc.ReduceAllUserBalances(context.Background(), "2.50000000")
	require.NoError(t, err)
	require.Equal(t, "2.5", repo.reduceFactor)
	require.Len(t, repo.operationID, 32)
	require.Equal(t, repo.operationID, result.OperationID)
}

func TestAdminServiceAllUserBalanceReductionRejectsInvalidFactors(t *testing.T) {
	svc := &adminServiceImpl{userRepo: &balanceReductionRepoStub{userRepoStub: &userRepoStub{}}}
	for _, factor := range []string{"", "1", "0.5", "NaN", "2.123456789"} {
		_, err := svc.PreviewAllUserBalanceReduction(context.Background(), factor)
		require.Error(t, err, factor)
	}
}
