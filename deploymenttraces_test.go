package api

import "testing"

func TestDeploymentTracesValid(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   *DeploymentTraces
		ok   bool
	}{
		{"nil", nil, true},
		{"disabled ignores values", &DeploymentTraces{Retention: "1y", Rate: -1, SamplePct: 500}, true},
		{"enabled", &DeploymentTraces{Enabled: true, Retention: "30d", Rate: 500, SamplePct: 100}, true},
		{"min sample", &DeploymentTraces{Enabled: true, Retention: "7d", Rate: 500, SamplePct: 1}, true},
		{"unknown retention", &DeploymentTraces{Enabled: true, Retention: "1d", Rate: 500, SamplePct: 100}, false},
		{"zero rate", &DeploymentTraces{Enabled: true, Retention: "7d", SamplePct: 100}, false},
		{"rate over max", &DeploymentTraces{Enabled: true, Retention: "7d", Rate: DeploymentTracesMaxRate + 1, SamplePct: 100}, false},
		{"zero sample", &DeploymentTraces{Enabled: true, Retention: "7d", Rate: 500}, false},
		{"sample over 100", &DeploymentTraces{Enabled: true, Retention: "7d", Rate: 500, SamplePct: 101}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.in.Valid(); (err == nil) != tt.ok {
				t.Fatalf("Valid() = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestDeploymentTracesWithDefaults(t *testing.T) {
	got := (&DeploymentTraces{Enabled: true}).WithDefaults()
	if got.Retention != DeploymentLogsDefaultRetention || got.Rate != DeploymentTracesDefaultRate || got.SamplePct != DeploymentTracesDefaultSamplePct {
		t.Fatalf("got %+v", got)
	}

	in := &DeploymentTraces{Enabled: true, Retention: "90d", Rate: 10, SamplePct: 5}
	if got := in.WithDefaults(); *got != *in {
		t.Fatalf("set values changed: %+v", got)
	}

	off := &DeploymentTraces{}
	if got := off.WithDefaults(); got != off || got.Retention != "" {
		t.Fatalf("disabled config changed: %+v", got)
	}
	if (*DeploymentTraces)(nil).WithDefaults() != nil {
		t.Fatal("nil should stay nil")
	}
}
