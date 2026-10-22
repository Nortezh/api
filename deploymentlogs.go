package api

import (
	"fmt"
	"slices"
)

const (
	DeploymentLogsDefaultRetention = "7d"
	DeploymentLogsDefaultRate      = 500
	DeploymentLogsMaxRate          = 5000
)

// DeploymentLogsRetentions are the retention tiers a deployment's log history can be kept for.
var DeploymentLogsRetentions = []string{"7d", "30d", "90d"}

// DeploymentLogs opts a deployment into log history. When enabled (and the location has the
// logs-v2 feature) the deployer labels the deployment's pods so their logs are shipped and kept
// for Retention, at most Rate log lines per second per pod.
type DeploymentLogs struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	Retention string `json:"retention" yaml:"retention"`
	Rate      int    `json:"rate" yaml:"rate"`
}

func (c *DeploymentLogs) Valid() error {
	if c == nil || !c.Enabled {
		return nil
	}
	if !slices.Contains(DeploymentLogsRetentions, c.Retention) {
		return fmt.Errorf("retention must be one of %v", DeploymentLogsRetentions)
	}
	if c.Rate < 1 || c.Rate > DeploymentLogsMaxRate {
		return fmt.Errorf("rate must be between 1 and %d", DeploymentLogsMaxRate)
	}
	return nil
}

// WithDefaults fills an enabled config's empty retention and rate with the defaults.
func (c *DeploymentLogs) WithDefaults() *DeploymentLogs {
	if c == nil || !c.Enabled {
		return c
	}
	x := *c
	if x.Retention == "" {
		x.Retention = DeploymentLogsDefaultRetention
	}
	if x.Rate == 0 {
		x.Rate = DeploymentLogsDefaultRate
	}
	return &x
}
