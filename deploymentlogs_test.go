package api

import "testing"

func TestDeploymentLogsValid(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   *DeploymentLogs
		ok   bool
	}{
		{"nil", nil, true},
		{"disabled ignores values", &DeploymentLogs{Retention: "1y", Rate: -1}, true},
		{"enabled", &DeploymentLogs{Enabled: true, Retention: "30d", Rate: 500}, true},
		{"max rate", &DeploymentLogs{Enabled: true, Retention: "90d", Rate: DeploymentLogsMaxRate}, true},
		{"unknown retention", &DeploymentLogs{Enabled: true, Retention: "1d", Rate: 500}, false},
		{"empty retention", &DeploymentLogs{Enabled: true, Rate: 500}, false},
		{"zero rate", &DeploymentLogs{Enabled: true, Retention: "7d"}, false},
		{"rate over max", &DeploymentLogs{Enabled: true, Retention: "7d", Rate: DeploymentLogsMaxRate + 1}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.in.Valid(); (err == nil) != tt.ok {
				t.Fatalf("Valid() = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestDeploymentLogsWithDefaults(t *testing.T) {
	got := (&DeploymentLogs{Enabled: true}).WithDefaults()
	if got.Retention != DeploymentLogsDefaultRetention || got.Rate != DeploymentLogsDefaultRate {
		t.Fatalf("got %+v", got)
	}

	in := &DeploymentLogs{Enabled: true, Retention: "90d", Rate: 10}
	if got := in.WithDefaults(); *got != *in {
		t.Fatalf("set values changed: %+v", got)
	}

	off := &DeploymentLogs{}
	if got := off.WithDefaults(); got != off || got.Retention != "" {
		t.Fatalf("disabled config changed: %+v", got)
	}

	if (*DeploymentLogs)(nil).WithDefaults() != nil {
		t.Fatal("nil should stay nil")
	}
}
