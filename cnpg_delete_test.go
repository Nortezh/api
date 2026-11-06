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
		if phase == CNPGDeleteFinalize {
			c.Receipt = cnpgDeleteReceipt()
		}
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
		r := *cnpgDeleteResult()
		r.Phase = phase
		if r.Valid() == nil {
			t.Fatal("phase-less result accepted")
		}
	}
}

func TestCNPGArchiveDeleteReceiptBoundary(t *testing.T) {
	for name, change := range map[string]func(*DeployerSetResultCNPGDelete){
		"legacy":          func(r *DeployerSetResultCNPGDelete) { r.Version = 2 },
		"missing create":  func(r *DeployerSetResultCNPGDelete) { r.CreateOperationID = "" },
		"wrong secret":    func(r *DeployerSetResultCNPGDelete) { r.SecretID = 0 },
		"wrong revision":  func(r *DeployerSetResultCNPGDelete) { r.SecretRevision = 0 },
		"resource only":   func(r *DeployerSetResultCNPGDelete) { r.Receipt = nil },
		"inventory":       func(r *DeployerSetResultCNPGDelete) { r.Receipt.InventoryUID = "" },
		"cluster":         func(r *DeployerSetResultCNPGDelete) { r.Receipt.ClusterUID = "00000000-0000-0000-0000-000000000000" },
		"same UID":        func(r *DeployerSetResultCNPGDelete) { r.Receipt.InventoryUID = r.Receipt.ClusterUID },
		"scope hash":      func(r *DeployerSetResultCNPGDelete) { r.Receipt.ArchiveBindingHash = strings.Repeat("A", 64) },
		"writer unknown":  func(r *DeployerSetResultCNPGDelete) { r.Receipt.WriterStopped = false },
		"archive unknown": func(r *DeployerSetResultCNPGDelete) { r.Receipt.ArchiveAbsent = false },
		"success stage":   func(r *DeployerSetResultCNPGDelete) { r.FailureStage = CNPGDeleteArchives },
		"success error":   func(r *DeployerSetResultCNPGDelete) { r.FailureCode = CNPGFailureCleanup },
	} {
		t.Run(name, func(t *testing.T) {
			r := cnpgDeleteResult()
			change(r)
			if r.Valid() == nil {
				t.Fatal("unsafe archive result accepted")
			}
		})
	}
	for _, phase := range []string{CNPGDeleteResources, CNPGDeleteFinalize} {
		for _, stage := range []string{CNPGDeleteScope, CNPGDeleteWriters, CNPGDeleteArchives, CNPGDeleteResources, CNPGDeleteFinalize, "", "raw provider error"} {
			r := cnpgDeleteResult()
			r.Phase, r.ResourceAbsent, r.FailureCode, r.FailureStage = phase, false, CNPGFailureCleanup, stage
			if phase == CNPGDeleteResources {
				r.Receipt = nil
			}
			want := phase == CNPGDeleteResources && (stage == CNPGDeleteScope || stage == CNPGDeleteWriters || stage == CNPGDeleteArchives || stage == CNPGDeleteResources) || phase == CNPGDeleteFinalize && stage == CNPGDeleteFinalize
			if (r.Valid() == nil) != want {
				t.Fatalf("unexpected failure boundary: phase=%s stage=%s", phase, stage)
			}
		}
	}
	c := cnpgDelete()
	c.Receipt = cnpgDeleteReceipt()
	if c.Valid() == nil {
		t.Fatal("resources command adopted a completed receipt")
	}
	c.Phase = CNPGDeleteFinalize
	if c.Valid() != nil {
		t.Fatal("finalize receipt rejected")
	}
	c.Receipt = nil
	if c.Valid() == nil {
		t.Fatal("finalize without accepted receipt")
	}
	for _, version := range []int{1, 2} {
		c := cnpgDelete()
		c.Version = version
		if c.Valid() == nil {
			t.Fatal("archive-unaware command became executable")
		}
	}
}
