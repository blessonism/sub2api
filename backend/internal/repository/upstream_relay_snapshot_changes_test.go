package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildRelaySnapshotChangesSkipsInitialBaseline(t *testing.T) {
	changes := buildRelaySnapshotChanges(10, nil, []service.UpstreamRelayGroupRateSnapshot{
		{UpstreamGroupID: "fast", Name: "Fast", FinalRateMultiplier: 1.2, Status: "active", Source: service.UpstreamRelayRateSourceAvailable},
	})

	require.Empty(t, changes)
}

func TestBuildRelaySnapshotChangesDetectsRateAddedAndRemoved(t *testing.T) {
	previous := []service.UpstreamRelayGroupRateSnapshot{
		{UpstreamGroupID: "stable", Name: "Stable", Platform: "openai", Status: "active", FinalRateMultiplier: 1.000000001, Source: service.UpstreamRelayRateSourceAvailable},
		{UpstreamGroupID: "changed", Name: "Changed", Platform: "openai", Status: "active", FinalRateMultiplier: 1.25, Source: service.UpstreamRelayRateSourceAvailable},
		{UpstreamGroupID: "removed", Name: "Removed", Platform: "anthropic", Status: "active", FinalRateMultiplier: 2, Source: service.UpstreamRelayRateSourceOverride},
		{UpstreamGroupID: "already-stale", Name: "Already Stale", Status: "stale", FinalRateMultiplier: 9, Source: service.UpstreamRelayRateSourceAvailable},
	}
	next := []service.UpstreamRelayGroupRateSnapshot{
		{UpstreamGroupID: "stable", Name: "Stable", Platform: "openai", Status: "active", FinalRateMultiplier: 1.000000002, Source: service.UpstreamRelayRateSourceAvailable},
		{UpstreamGroupID: "changed", Name: "Changed", Platform: "openai", Status: "active", FinalRateMultiplier: 1.5, Source: service.UpstreamRelayRateSourceOverride},
		{UpstreamGroupID: "added", Name: "Added", Platform: "gemini", Status: "active", FinalRateMultiplier: 0.8, Source: service.UpstreamRelayRateSourceAvailable},
	}

	changes := buildRelaySnapshotChanges(10, previous, next)

	require.Len(t, changes, 3)
	require.Equal(t, service.UpstreamRelaySnapshotChangeRateChanged, changes[0].ChangeType)
	require.Equal(t, "changed", changes[0].UpstreamGroupID)
	require.NotNil(t, changes[0].OldFinalRateMultiplier)
	require.NotNil(t, changes[0].NewFinalRateMultiplier)
	require.Equal(t, 1.25, *changes[0].OldFinalRateMultiplier)
	require.Equal(t, 1.5, *changes[0].NewFinalRateMultiplier)
	require.Equal(t, service.UpstreamRelaySnapshotChangeAdded, changes[1].ChangeType)
	require.Equal(t, "added", changes[1].UpstreamGroupID)
	require.Nil(t, changes[1].OldFinalRateMultiplier)
	require.Equal(t, service.UpstreamRelaySnapshotChangeRemoved, changes[2].ChangeType)
	require.Equal(t, "removed", changes[2].UpstreamGroupID)
	require.NotNil(t, changes[2].OldFinalRateMultiplier)
	require.Nil(t, changes[2].NewFinalRateMultiplier)
}

func TestBuildRelaySnapshotChangesDetectsStaleGroupRestoredAsAdded(t *testing.T) {
	previous := []service.UpstreamRelayGroupRateSnapshot{
		{UpstreamGroupID: "restored", Name: "Restored", Platform: "openai", Status: "stale", FinalRateMultiplier: 0.8, Source: service.UpstreamRelayRateSourceAvailable},
	}
	next := []service.UpstreamRelayGroupRateSnapshot{
		{UpstreamGroupID: "restored", Name: "Restored", Platform: "openai", Status: "active", FinalRateMultiplier: 0.8, Source: service.UpstreamRelayRateSourceAvailable},
	}

	changes := buildRelaySnapshotChanges(10, previous, next)

	require.Len(t, changes, 1)
	require.Equal(t, service.UpstreamRelaySnapshotChangeAdded, changes[0].ChangeType)
	require.Equal(t, "restored", changes[0].UpstreamGroupID)
	require.Equal(t, "stale", changes[0].OldStatus)
	require.Equal(t, "active", changes[0].NewStatus)
	require.NotNil(t, changes[0].OldFinalRateMultiplier)
	require.NotNil(t, changes[0].NewFinalRateMultiplier)
	require.Equal(t, 0.8, *changes[0].OldFinalRateMultiplier)
	require.Equal(t, 0.8, *changes[0].NewFinalRateMultiplier)
}
