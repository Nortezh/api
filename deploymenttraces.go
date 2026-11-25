package api

import (
	"fmt"
	"slices"
)

const (
	DeploymentTracesDefaultRate      = 500
	DeploymentTracesMaxRate          = 5000
	DeploymentTracesDefaultSamplePct = 100
)

// DeploymentTraces opts a deployment into tracing. When enabled (and the location has the traces
// feature) the deployer labels the deployment's pods so the spans they send are shipped and kept
// for Retention, at most Rate spans per second per pod, keeping SamplePct percent of traces (whole
// traces, chosen by trace id). Retention is shared with the deployment's log history: a pod has one
// retention for everything it ships.
type DeploymentTraces struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	Retention string `json:"retention" yaml:"retention"`
	Rate      int    `json:"rate" yaml:"rate"`
	SamplePct int    `json:"samplePct" yaml:"samplePct"`
}

func (c *DeploymentTraces) Valid() error {
	if c == nil || !c.Enabled {
		return nil
	}
	if !slices.Contains(DeploymentLogsRetentions, c.Retention) {
		return fmt.Errorf("retention must be one of %v", DeploymentLogsRetentions)
	}
	if c.Rate < 1 || c.Rate > DeploymentTracesMaxRate {
		return fmt.Errorf("rate must be between 1 and %d", DeploymentTracesMaxRate)
	}
	if c.SamplePct < 1 || c.SamplePct > 100 {
		return fmt.Errorf("samplePct must be between 1 and 100")
	}
	return nil
}

// WithDefaults fills an enabled config's empty retention, rate and sample percent with the defaults.
func (c *DeploymentTraces) WithDefaults() *DeploymentTraces {
	if c == nil || !c.Enabled {
		return c
	}
	x := *c
	if x.Retention == "" {
		x.Retention = DeploymentLogsDefaultRetention
	}
	if x.Rate == 0 {
		x.Rate = DeploymentTracesDefaultRate
	}
	if x.SamplePct == 0 {
		x.SamplePct = DeploymentTracesDefaultSamplePct
	}
	return &x
}
