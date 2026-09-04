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

func TestBuildStatusSummaryCopiesErrorCategory(t *testing.T) {
	latest := map[string]*ChannelMonitorLatest{
		"primary": {
			Model:           "primary",
			Status:          MonitorStatusError,
			EffectiveStatus: MonitorStatusError,
			ErrorCategory:   MonitorErrorCategoryRateOrCapacity,
		},
		"extra": {
			Model:           "extra",
			Status:          MonitorStatusError,
			EffectiveStatus: MonitorStatusError,
			ErrorCategory:   MonitorErrorCategoryRateOrCapacity,
		},
	}

	summary := buildStatusSummary(latest, nil, "primary", []string{"extra"})

	if summary.PrimaryErrorCategory != MonitorErrorCategoryRateOrCapacity {
		t.Fatalf("expected primary error category, got %q", summary.PrimaryErrorCategory)
	}
	if len(summary.ExtraModels) != 1 || summary.ExtraModels[0].ErrorCategory != MonitorErrorCategoryRateOrCapacity {
		t.Fatalf("expected extra model error category, got %#v", summary.ExtraModels)
	}
}

func TestBuildUserViewFromSummaryCopiesTargetKindAndErrorCategory(t *testing.T) {
	view := buildUserViewFromSummary(
		&ChannelMonitor{
			ID:           7,
			Name:         "pro",
			Provider:     MonitorProviderOpenAI,
			PrimaryModel: "gpt-5.6",
			TargetKind:   MonitorTargetKindGatewayGroup,
		},
		MonitorStatusSummary{
			PrimaryStatus:        MonitorStatusError,
			PrimaryErrorCategory: MonitorErrorCategoryRateOrCapacity,
		},
		nil,
		[]*ChannelMonitorHistoryEntry{{
			EffectiveStatus: MonitorStatusError,
			ErrorCategory:   MonitorErrorCategoryRateOrCapacity,
			CheckedAt:       time.Now(),
		}},
	)

	if view.TargetKind != MonitorTargetKindGatewayGroup {
		t.Fatalf("expected gateway_group target kind, got %q", view.TargetKind)
	}
	if view.PrimaryErrorCategory != MonitorErrorCategoryRateOrCapacity {
		t.Fatalf("expected primary error category, got %q", view.PrimaryErrorCategory)
	}
	if len(view.Timeline) != 1 || view.Timeline[0].ErrorCategory != MonitorErrorCategoryRateOrCapacity {
		t.Fatalf("expected timeline error category, got %#v", view.Timeline)
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
	if points[0].ErrorCategory != "" {
		t.Fatalf("operational override should not inherit error category unless present, got %q", points[0].ErrorCategory)
	}
}

func monitorIntPtr(v int) *int {
	return &v
}
