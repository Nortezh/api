package api

import (
	"fmt"
	"strings"
)

// GoogleAuthConfig gates a deployment's external ingress behind Google login,
// allowing only the emails/domains on the allowlist. Reconciled by the deployer
// into a parapet forward-auth annotation pointing at the authgate verifier.
type GoogleAuthConfig struct {
	Enabled        bool     `json:"enabled" yaml:"enabled"`
	AllowedEmails  []string `json:"allowedEmails" yaml:"allowedEmails"`
	AllowedDomains []string `json:"allowedDomains" yaml:"allowedDomains"`
}

func (c *GoogleAuthConfig) Valid() error {
	if c == nil || !c.Enabled {
		return nil
	}
	if len(c.AllowedEmails) == 0 && len(c.AllowedDomains) == 0 {
		return fmt.Errorf("allowlist must not be empty when enabled")
	}
	if len(c.AllowedEmails)+len(c.AllowedDomains) > 200 {
		return fmt.Errorf("allowlist too large (max 200)")
	}
	for _, e := range c.AllowedEmails {
		if !strings.Contains(e, "@") || strings.HasPrefix(e, "@") {
			return fmt.Errorf("invalid email in allowedEmails: %s", e)
		}
	}
	for _, d := range c.AllowedDomains {
		if d == "" || strings.ContainsAny(d, "@ ") || !strings.Contains(d, ".") {
			return fmt.Errorf("invalid domain in allowedDomains: %s", d)
		}
	}
	return nil
}

// Allow returns the flattened allowlist for the verifier ?allow= query:
// bare lowercase emails plus "@"-prefixed lowercase domains.
func (c *GoogleAuthConfig) Allow() []string {
	out := make([]string, 0, len(c.AllowedEmails)+len(c.AllowedDomains))
	for _, e := range c.AllowedEmails {
		out = append(out, strings.ToLower(strings.TrimSpace(e)))
	}
	for _, d := range c.AllowedDomains {
		out = append(out, "@"+strings.ToLower(strings.TrimSpace(d)))
	}
	return out
}
