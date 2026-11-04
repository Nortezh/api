package api

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func retentionUUID(n int) string { return fmt.Sprintf("%08x-1111-4111-8111-111111111111", n) }

func retentionCommand() DeployerCommandCNPGBackupRetention {
	m := DeployerCommandCNPGBackupRetention{
		DeployerCommandCNPGIdentity: cnpgIdentity(), CreateOperationID: retentionUUID(100), SourceClusterUID: retentionUUID(101),
	}
	for i := 0; i < 8; i++ {
		material := *cnpgBackupResult().Material
		material.BarmanBackupID = fmt.Sprintf("202611%02dT020000", i+1)
		material.ServerName = fmt.Sprintf("cnpg-%d-%d", m.ID, m.ProjectID)
		material.Archive = &CNPGBackupArchive{EndpointURL: "http://minio.test:9000", DestinationPath: "s3://backups/prefix",
			ProfileRevision: 1, ConfigurationHash: strings.Repeat("a", 64)}
		m.Backups = append(m.Backups, CNPGBackupRetentionItem{OperationID: retentionUUID(i + 1), BackupUID: retentionUUID(i + 20), Material: material})
	}
	m.ReplacementOperationID = m.Backups[7].OperationID
	return m
}

func retentionResult() DeployerSetResultCNPGBackupRetention {
	m := retentionCommand()
	m.JobUID = retentionUUID(200)
	r := DeployerSetResultCNPGBackupRetention{DeployerCommandCNPGBackupRetention: m, Status: CNPGBackupRetentionCompleted,
		PodUIDs: []string{retentionUUID(201)}, ExpiredIDs: []string{m.Backups[0].Material.BarmanBackupID}}
	for _, b := range m.Backups[1:] {
		r.RetainedIDs = append(r.RetainedIDs, b.Material.BarmanBackupID)
	}
	return r
}

func TestRetentionCommandValidation(t *testing.T) {
	if err := retentionCommand().Valid(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*DeployerCommandCNPGBackupRetention){
		"version": func(m *DeployerCommandCNPGBackupRetention) { m.Version = 2 },
		"source":  func(m *DeployerCommandCNPGBackupRetention) { m.SourceClusterUID = "" },
		"create":  func(m *DeployerCommandCNPGBackupRetention) { m.CreateOperationID = m.OperationID },
		"job":     func(m *DeployerCommandCNPGBackupRetention) { m.JobUID = "invalid" },
		"short":   func(m *DeployerCommandCNPGBackupRetention) { m.Backups = m.Backups[:7] },
		"long":    func(m *DeployerCommandCNPGBackupRetention) { m.Backups = append(m.Backups, m.Backups[0]) },
		"huge":    func(m *DeployerCommandCNPGBackupRetention) { m.Backups = make([]CNPGBackupRetentionItem, 10000) },
		"unbound": func(m *DeployerCommandCNPGBackupRetention) { m.Backups[0].Material.Archive = nil },
		"duplicate operation": func(m *DeployerCommandCNPGBackupRetention) {
			m.Backups[1].OperationID = strings.ToUpper(m.Backups[0].OperationID)
		},
		"retention as backup":  func(m *DeployerCommandCNPGBackupRetention) { m.Backups[0].OperationID = m.OperationID },
		"duplicate native UID": func(m *DeployerCommandCNPGBackupRetention) { m.Backups[1].BackupUID = m.Backups[0].BackupUID },
		"bad UID":              func(m *DeployerCommandCNPGBackupRetention) { m.Backups[0].BackupUID = "" },
		"order":                func(m *DeployerCommandCNPGBackupRetention) { m.Backups[0], m.Backups[1] = m.Backups[1], m.Backups[0] },
		"duplicate native ID": func(m *DeployerCommandCNPGBackupRetention) {
			m.Backups[1].Material.BarmanBackupID = m.Backups[0].Material.BarmanBackupID
		},
		"replacement":     func(m *DeployerCommandCNPGBackupRetention) { m.ReplacementOperationID = m.Backups[0].OperationID },
		"foreign project": func(m *DeployerCommandCNPGBackupRetention) { m.ProjectID++ },
		"foreign source":  func(m *DeployerCommandCNPGBackupRetention) { m.ID++ },
		"server":          func(m *DeployerCommandCNPGBackupRetention) { m.Backups[0].Material.ServerName = "foreign" },
		"store":           func(m *DeployerCommandCNPGBackupRetention) { m.Backups[1].Material.ObjectStoreUID = retentionUUID(500) },
		"image": func(m *DeployerCommandCNPGBackupRetention) {
			m.Backups[1].Material.Image = "other@sha256:" + strings.Repeat("b", 64)
		},
		"archive": func(m *DeployerCommandCNPGBackupRetention) {
			m.Backups[1].Material.Archive.DestinationPath = "s3://other/prefix"
		},
		"profile": func(m *DeployerCommandCNPGBackupRetention) { m.Backups[1].Material.Archive.ProfileRevision++ },
	} {
		t.Run(name, func(t *testing.T) {
			m := retentionCommand()
			change(&m)
			if m.Valid() == nil {
				t.Fatal("invalid retention command accepted")
			}
		})
	}
}

func TestArchiveBindingValidationAndLegacyCompatibility(t *testing.T) {
	legacy := cnpgBackupResult()
	if err := legacy.Valid(); err != nil {
		t.Fatal("legacy material rejected", err)
	}
	material := retentionCommand().Backups[0].Material
	if err := material.Valid(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*CNPGBackupArchive){
		"endpoint credentials":    func(m *CNPGBackupArchive) { m.EndpointURL = "https://private:password@store.test" },
		"query":                   func(m *CNPGBackupArchive) { m.EndpointURL += "?token=private" },
		"empty query":             func(m *CNPGBackupArchive) { m.EndpointURL += "?" },
		"fragment":                func(m *CNPGBackupArchive) { m.EndpointURL += "#private" },
		"endpoint path":           func(m *CNPGBackupArchive) { m.EndpointURL += "/other" },
		"endpoint scheme":         func(m *CNPGBackupArchive) { m.EndpointURL = "file:///tmp/store" },
		"endpoint length":         func(m *CNPGBackupArchive) { m.EndpointURL = "https://" + strings.Repeat("x", 2049) },
		"destination credentials": func(m *CNPGBackupArchive) { m.DestinationPath = "s3://private@bucket/prefix" },
		"destination query":       func(m *CNPGBackupArchive) { m.DestinationPath += "?private" },
		"destination scheme":      func(m *CNPGBackupArchive) { m.DestinationPath = "https://bucket/prefix" },
		"destination port":        func(m *CNPGBackupArchive) { m.DestinationPath = "s3://bucket:9000/prefix" },
		"traversal":               func(m *CNPGBackupArchive) { m.DestinationPath = "s3://bucket/prefix/../other" },
		"encoded traversal":       func(m *CNPGBackupArchive) { m.DestinationPath = "s3://bucket/%2e%2e/other" },
		"encoded slash":           func(m *CNPGBackupArchive) { m.DestinationPath = "s3://bucket/prefix%2fother" },
		"empty segment":           func(m *CNPGBackupArchive) { m.DestinationPath = "s3://bucket/prefix//other" },
		"backslash":               func(m *CNPGBackupArchive) { m.DestinationPath = "s3://bucket/prefix\\other" },
		"profile":                 func(m *CNPGBackupArchive) { m.ProfileRevision = 0 },
		"hash":                    func(m *CNPGBackupArchive) { m.ConfigurationHash = strings.Repeat("g", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			m := *material.Archive
			change(&m)
			if err := m.Valid(); err == nil || strings.Contains(err.Error(), "private") {
				t.Fatal("accepted unsafe archive or leaked input", err)
			}
		})
	}
}

func TestRetentionResultValidation(t *testing.T) {
	for name, change := range map[string]func(*DeployerSetResultCNPGBackupRetention){
		"job missing":                    func(m *DeployerSetResultCNPGBackupRetention) { m.JobUID = "" },
		"pods missing":                   func(m *DeployerSetResultCNPGBackupRetention) { m.PodUIDs = nil },
		"multiple pods":                  func(m *DeployerSetResultCNPGBackupRetention) { m.PodUIDs = append(m.PodUIDs, retentionUUID(202)) },
		"duplicate pods":                 func(m *DeployerSetResultCNPGBackupRetention) { m.PodUIDs = append(m.PodUIDs, m.PodUIDs[0]) },
		"oversized pods":                 func(m *DeployerSetResultCNPGBackupRetention) { m.PodUIDs = make([]string, 10000) },
		"retained missing":               func(m *DeployerSetResultCNPGBackupRetention) { m.RetainedIDs = nil },
		"replacement missing":            func(m *DeployerSetResultCNPGBackupRetention) { m.RetainedIDs[6] = m.Backups[0].Material.BarmanBackupID },
		"wrong expired":                  func(m *DeployerSetResultCNPGBackupRetention) { m.ExpiredIDs[0] = m.RetainedIDs[0] },
		"duplicate retained":             func(m *DeployerSetResultCNPGBackupRetention) { m.RetainedIDs[1] = m.RetainedIDs[0] },
		"raw failure":                    func(m *DeployerSetResultCNPGBackupRetention) { m.FailureCode = "private raw details" },
		"running with terminal evidence": func(m *DeployerSetResultCNPGBackupRetention) { m.Status = CNPGBackupRetentionRunning },
		"blocked with terminal evidence": func(m *DeployerSetResultCNPGBackupRetention) {
			m.Status = CNPGBackupRetentionBlocked
			m.FailureCode = CNPGFailureRetentionJob
		},
		"unknown": func(m *DeployerSetResultCNPGBackupRetention) { m.Status = "unknown" },
	} {
		t.Run(name, func(t *testing.T) {
			m := retentionResult()
			change(&m)
			if err := m.Valid(); err == nil || strings.Contains(err.Error(), "private") {
				t.Fatal("invalid result accepted or echoed input", err)
			}
		})
	}
}

func TestRetentionEnvelopeRoundTripAndExclusivity(t *testing.T) {
	completed := retentionResult()
	running := completed
	running.Status, running.RetainedIDs, running.ExpiredIDs = CNPGBackupRetentionRunning, nil, nil
	blocked := running
	blocked.Status, blocked.FailureCode, blocked.JobUID, blocked.PodUIDs = CNPGBackupRetentionBlocked, CNPGFailureRetentionScope, "", nil
	command := retentionCommand()
	for _, item := range []any{
		&DeployerCommandItem{CNPGBackupRetention: &command},
		&DeployerSetResultItem{CNPGBackupRetention: &completed},
		&DeployerSetResultItem{CNPGBackupRetention: &running},
		&DeployerSetResultItem{CNPGBackupRetention: &blocked},
	} {
		wire, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"password", "secretId", "command", "args", "failureMessage", "walCleanupVerified"} {
			if strings.Contains(string(wire), `"`+field+`"`) {
				t.Fatal("forbidden retention field", field)
			}
		}
		decoded := reflect.New(reflect.TypeOf(item).Elem()).Interface()
		if err := json.Unmarshal(wire, decoded); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(item, decoded) || decoded.(interface{ ValidCNPG() error }).ValidCNPG() != nil {
			t.Fatal("invalid retention round trip")
		}
		typ := reflect.TypeOf(item).Elem()
		for i := 0; i < typ.NumField(); i++ {
			mixed := reflect.New(typ)
			mixed.Elem().Set(reflect.ValueOf(item).Elem())
			if typ.Field(i).Name != "CNPGBackupRetention" {
				mixed.Elem().Field(i).Set(reflect.New(typ.Field(i).Type.Elem()))
			}
			want := typ.Field(i).Name == "CNPGBackupRetention"
			if (mixed.Interface().(interface{ ValidCNPG() error }).ValidCNPG() == nil) != want {
				t.Fatal("mixed retention envelope accepted", typ.Field(i).Name)
			}
		}
	}
}
