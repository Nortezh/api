package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCNPGDeletePhaseBoundary(t *testing.T) {
	for _, phase := range []string{CNPGDeleteResources, CNPGDeleteFinalize} {
		c := *cnpgDelete()
		c.Phase = phase
		if c.Valid() != nil {
			t.Fatal("valid delete rejected")
		}
	}
	for _, change := range []func(*DeployerCommandCNPGDelete){
		func(c *DeployerCommandCNPGDelete) { c.Phase = "" }, func(c *DeployerCommandCNPGDelete) { c.Phase = "force" },
		func(c *DeployerCommandCNPGDelete) { c.SecretID = 0 }, func(c *DeployerCommandCNPGDelete) { c.SecretRevision = 0 },
		func(c *DeployerCommandCNPGDelete) { c.CreateOperationID = "" }, func(c *DeployerCommandCNPGDelete) { c.CreateOperationID = "00000000-0000-0000-0000-000000000000" },
		func(c *DeployerCommandCNPGDelete) { c.CreateOperationID = strings.ToUpper(c.OperationID) },
	} {
		c := *cnpgDelete()
		change(&c)
		if c.Valid() == nil {
			t.Fatal("unsafe delete accepted")
		}
	}
	var c DeployerCommandItem
	if err := json.Unmarshal([]byte(`{"cnpgDelete":{"version":1,"id":1,"projectId":2,"operationId":"a1111111-1111-4111-8111-111111111111"}}`), &c); err != nil {
		t.Fatal(err)
	}
	if c.ValidCNPG() == nil {
		t.Fatal("published identity-only delete became executable")
	}
	for _, phase := range []string{"", "force"} {
		r := DeployerSetResultCNPGDelete{DeployerCommandCNPGIdentity: cnpgIdentity(), Phase: phase, ResourceAbsent: true}
		if r.Valid() == nil {
			t.Fatal("phase-less result accepted")
		}
	}
}
