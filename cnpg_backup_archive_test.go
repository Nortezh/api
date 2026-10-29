package api

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func archiveBindingFixture() CNPGBackupArchiveBinding {
	return CNPGBackupArchiveBinding{Format: 1, EndpointURL: "https://archive.example", DestinationPath: "s3://test-bucket/base", BeginWAL: "000000010000000000000002", Timeline: 1}
}

func TestCNPGBackupArchiveBindingValidation(t *testing.T) {
	material := cnpgBackupResult().Material
	good := archiveBindingFixture()
	if good.Valid(material) != nil || good.Valid(nil) == nil {
		t.Fatal("binding/material validation")
	}
	for name, mutate := range map[string]func(*CNPGBackupArchiveBinding){
		"format":                func(b *CNPGBackupArchiveBinding) { b.Format = 2 },
		"timeline":              func(b *CNPGBackupArchiveBinding) { b.Timeline = 2 },
		"zero timeline":         func(b *CNPGBackupArchiveBinding) { b.Timeline = 0 },
		"credentials":           func(b *CNPGBackupArchiveBinding) { b.EndpointURL = "https://user:PRIVATE@archive.example" },
		"query":                 func(b *CNPGBackupArchiveBinding) { b.EndpointURL += "?key=PRIVATE" },
		"empty query":           func(b *CNPGBackupArchiveBinding) { b.EndpointURL += "?" },
		"endpoint path":         func(b *CNPGBackupArchiveBinding) { b.EndpointURL += "/foreign" },
		"endpoint escaped path": func(b *CNPGBackupArchiveBinding) { b.EndpointURL += "/%2Fforeign" },
		"endpoint scheme":       func(b *CNPGBackupArchiveBinding) { b.EndpointURL = "file:///tmp/archive" },
		"invalid URL":           func(b *CNPGBackupArchiveBinding) { b.EndpointURL = "https://[invalid" },
		"traversal":             func(b *CNPGBackupArchiveBinding) { b.DestinationPath = "s3://bucket/base/../foreign" },
		"escaped path":          func(b *CNPGBackupArchiveBinding) { b.DestinationPath = "s3://bucket/base%2Fforeign" },
		"destination scheme":    func(b *CNPGBackupArchiveBinding) { b.DestinationPath = "https://bucket/base" },
		"wildcard path":         func(b *CNPGBackupArchiveBinding) { b.DestinationPath += "*" },
		"wildcard bucket":       func(b *CNPGBackupArchiveBinding) { b.DestinationPath = "s3://*/base" },
		"invalid bucket":        func(b *CNPGBackupArchiveBinding) { b.DestinationPath = "s3://a..b/base" },
		"destination query":     func(b *CNPGBackupArchiveBinding) { b.DestinationPath += "?" },
		"later WAL":             func(b *CNPGBackupArchiveBinding) { b.BeginWAL = "000000010000000000000009" },
		"different timeline":    func(b *CNPGBackupArchiveBinding) { b.BeginWAL = "000000020000000000000002"; b.Timeline = 2 },
		"bad WAL":               func(b *CNPGBackupArchiveBinding) { b.BeginWAL = "bad" },
	} {
		t.Run(name, func(t *testing.T) {
			b := good
			mutate(&b)
			err := b.Valid(material)
			if err == nil || strings.Contains(err.Error(), "PRIVATE") {
				t.Fatal("invalid binding accepted or details leaked")
			}
		})
	}
}

func TestCNPGBackupOptionalArchiveBindingWire(t *testing.T) {
	r := cnpgBackupResult()
	if r.Valid() != nil {
		t.Fatal("legacy completion rejected")
	}
	b := archiveBindingFixture()
	r.ArchiveBinding = &b
	wire, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded DeployerSetResultCNPGBackup
	if json.Unmarshal(wire, &decoded) != nil || !reflect.DeepEqual(r, decoded) || decoded.Valid() != nil {
		t.Fatal("archive binding wire lost evidence")
	}
	for _, status := range []CNPGBackupStatus{CNPGBackupRunning, CNPGBackupFailed} {
		r.Status, r.Material = status, nil
		if r.Valid() == nil {
			t.Fatal("noncompleted binding accepted")
		}
	}
}
