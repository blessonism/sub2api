//go:build unit

package service

import (
	"testing"
	"time"
)

func monitorStatusPtr(s string) *string {
	return &s
}

func TestBuildStatusSummaryUsesEffectiveStatus(t *testing.T) {
	latest := map[string]*ChannelMonitorLatest{
		"primary": {
			Model:           "primary",
			Status:          MonitorStatusFailed,
			OverrideStatus:  monitorStatusPtr(MonitorStatusOperational),
			EffectiveStatus: MonitorStatusOperational,
			LatencyMs:       monitorIntPtr(120),
		},
		"extra": {
			Model:           "extra",
			Status:          MonitorStatusOperational,
			OverrideStatus:  monitorStatusPtr(MonitorStatusFailed),
			EffectiveStatus: MonitorStatusFailed,
			LatencyMs:       monitorIntPtr(300),
		},
	}
	availability := map[string]*ChannelMonitorAvailability{
		"primary": {AvailabilityPct: 88.8},
	}

	summary := buildStatusSummary(latest, availability, "primary", []string{"extra"})

	if summary.PrimaryStatus != MonitorStatusOperational {
		t.Fatalf("expected primary effective status, got %q", summary.PrimaryStatus)
	}
	if summary.Availability7d != 88.8 {
		t.Fatalf("expected availability to pass through, got %.2f", summary.Availability7d)
	}
	if len(summary.ExtraModels) != 1 || summary.ExtraModels[0].Status != MonitorStatusFailed {
		t.Fatalf("expected extra model effective failed status, got %#v", summary.ExtraModels)
	}
}

func TestBuildTimelinePointsUsesEffectiveStatus(t *testing.T) {
	points := buildTimelinePoints([]*ChannelMonitorHistoryEntry{
		{
			Status:          MonitorStatusFailed,
			OverrideStatus:  monitorStatusPtr(MonitorStatusOperational),
			EffectiveStatus: MonitorStatusOperational,
			CheckedAt:       time.Now(),
		},
	})

	if len(points) != 1 {
		t.Fatalf("expected 1 timeline point, got %d", len(points))
	}
	if points[0].Status != MonitorStatusOperational {
		t.Fatalf("expected timeline to use effective status, got %q", points[0].Status)
	}
}

func monitorIntPtr(v int) *int {
	return &v
}
