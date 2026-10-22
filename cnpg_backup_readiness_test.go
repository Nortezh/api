package api

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func readinessCommand() DeployerCommandCNPGBackupReadiness {
	return DeployerCommandCNPGBackupReadiness{
		DeployerCommandCNPGIdentity: cnpgIdentity(),
		CheckID:                     "b1111111-1111-4111-8111-111111111111", ProfileRevision: 1,
	}
}

func readinessResult() DeployerSetResultCNPGBackupReadiness {
	return DeployerSetResultCNPGBackupReadiness{
		DeployerCommandCNPGBackupReadiness: readinessCommand(), Available: true,
		Evidence: &CNPGBackupReadinessEvidence{
			SourceClusterUID: "c1111111-1111-4111-8111-111111111111",
			ObjectStoreName:  "backup-store", ObjectStoreUID: "d1111111-1111-4111-8111-111111111111",
			ServerName: "cnpg-1-2", ConfigurationHash: strings.Repeat("ab", 32),
		},
	}
}

func TestCNPGBackupReadinessCommandValidation(t *testing.T) {
	for name, mutate := range map[string]func(*DeployerCommandCNPGBackupReadiness){
		"version":           func(m *DeployerCommandCNPGBackupReadiness) { m.Version = 2 },
		"database":          func(m *DeployerCommandCNPGBackupReadiness) { m.ID = 0 },
		"project":           func(m *DeployerCommandCNPGBackupReadiness) { m.ProjectID = 0 },
		"operation":         func(m *DeployerCommandCNPGBackupReadiness) { m.OperationID = "bad" },
		"empty check":       func(m *DeployerCommandCNPGBackupReadiness) { m.CheckID = "" },
		"malformed check":   func(m *DeployerCommandCNPGBackupReadiness) { m.CheckID = "bad" },
		"zero check":        func(m *DeployerCommandCNPGBackupReadiness) { m.CheckID = "00000000-0000-0000-0000-000000000000" },
		"same operation":    func(m *DeployerCommandCNPGBackupReadiness) { m.CheckID = strings.ToUpper(m.OperationID) },
		"missing revision":  func(m *DeployerCommandCNPGBackupReadiness) { m.ProfileRevision = 0 },
		"negative revision": func(m *DeployerCommandCNPGBackupReadiness) { m.ProfileRevision = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			m := readinessCommand()
			mutate(&m)
			if m.Valid() == nil {
				t.Fatal("invalid readiness command accepted")
			}
			if (&DeployerCommandItem{CNPGBackupReadiness: &m}).ValidCNPG() == nil {
				t.Fatal("invalid command bypassed envelope validation")
			}
		})
	}
}

func TestCNPGBackupReadinessEvidenceValidation(t *testing.T) {
	for name, mutate := range map[string]func(*CNPGBackupReadinessEvidence){
		"empty source":     func(m *CNPGBackupReadinessEvidence) { m.SourceClusterUID = "" },
		"zero source":      func(m *CNPGBackupReadinessEvidence) { m.SourceClusterUID = "00000000-0000-0000-0000-000000000000" },
		"bad source":       func(m *CNPGBackupReadinessEvidence) { m.SourceClusterUID = "bad" },
		"empty store":      func(m *CNPGBackupReadinessEvidence) { m.ObjectStoreName = "" },
		"bad store":        func(m *CNPGBackupReadinessEvidence) { m.ObjectStoreName = "https://store" },
		"empty store uid":  func(m *CNPGBackupReadinessEvidence) { m.ObjectStoreUID = "" },
		"zero store uid":   func(m *CNPGBackupReadinessEvidence) { m.ObjectStoreUID = "00000000-0000-0000-0000-000000000000" },
		"bad store uid":    func(m *CNPGBackupReadinessEvidence) { m.ObjectStoreUID = "bad" },
		"empty server":     func(m *CNPGBackupReadinessEvidence) { m.ServerName = "" },
		"prefix traversal": func(m *CNPGBackupReadinessEvidence) { m.ServerName = "../other" },
		"oversized server": func(m *CNPGBackupReadinessEvidence) { m.ServerName = strings.Repeat("a", 254) },
		"empty hash":       func(m *CNPGBackupReadinessEvidence) { m.ConfigurationHash = "" },
		"short hash":       func(m *CNPGBackupReadinessEvidence) { m.ConfigurationHash = "ab" },
		"nonhex hash":      func(m *CNPGBackupReadinessEvidence) { m.ConfigurationHash = strings.Repeat("z", 64) },
		"uppercase hash":   func(m *CNPGBackupReadinessEvidence) { m.ConfigurationHash = strings.Repeat("AB", 32) },
	} {
		t.Run(name, func(t *testing.T) {
			m := readinessResult()
			mutate(m.Evidence)
			if m.Valid() == nil || (&DeployerSetResultItem{CNPGBackupReadiness: &m}).ValidCNPG() == nil {
				t.Fatal("invalid readiness evidence accepted")
			}
		})
	}
}

func TestCNPGBackupReadinessResultShape(t *testing.T) {
	for _, tc := range []struct {
		name                string
		available, evidence bool
		code                CNPGFailureCode
		valid               bool
	}{
		{"ready", true, true, "", true},
		{"unavailable", false, false, CNPGFailureBackupReadiness, true},
		{"missing evidence", true, false, "", false},
		{"ready with error", true, true, CNPGFailureBackupReadiness, false},
		{"unavailable with evidence", false, true, CNPGFailureBackupReadiness, false},
		{"missing reason", false, false, "", false},
		{"native backup failure", false, false, CNPGFailureBackup, false},
		{"raw upstream error", false, false, "secret-value upstream error", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := readinessResult()
			m.Available, m.FailureCode = tc.available, tc.code
			if !tc.evidence {
				m.Evidence = nil
			}
			err := (&DeployerSetResultItem{CNPGBackupReadiness: &m}).ValidCNPG()
			if (err == nil) != tc.valid {
				t.Fatalf("unexpected validation result: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "secret-value") {
				t.Fatal("raw diagnostic echoed")
			}
		})
	}
	m := readinessResult()
	m.CheckID = ""
	if m.Valid() == nil {
		t.Fatal("result accepted without valid request identity")
	}
}

func TestCNPGBackupReadinessEnvelopeAndRoundTrip(t *testing.T) {
	command, result := readinessCommand(), readinessResult()
	unavailable := DeployerSetResultCNPGBackupReadiness{
		DeployerCommandCNPGBackupReadiness: command, FailureCode: CNPGFailureBackupReadiness,
	}
	for _, value := range []any{
		&DeployerCommandItem{CNPGBackupReadiness: &command},
		&DeployerSetResultItem{CNPGBackupReadiness: &result},
		&DeployerSetResultItem{CNPGBackupReadiness: &unavailable},
	} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(data, &envelope); err != nil || len(envelope) != 1 || envelope["cnpgBackupReadiness"] == nil {
			t.Fatal("readiness must use its own sole envelope")
		}
		for _, field := range []string{"password", "secret", "endpoint", "destinationPath", "failureMessage", "queryReady", "tlsReady", "backupUid", "status"} {
			if strings.Contains(string(data), `"`+field+`"`) {
				t.Fatalf("unexpected readiness field: %s", field)
			}
		}
		target := reflect.New(reflect.TypeOf(value).Elem()).Interface()
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(value, target) || target.(interface{ ValidCNPG() error }).ValidCNPG() != nil {
			t.Fatal("readiness round trip changed or invalid")
		}
		typ := reflect.TypeOf(value).Elem()
		for i := 0; i < typ.NumField(); i++ {
			mixed := reflect.New(typ)
			mixed.Elem().Set(reflect.ValueOf(value).Elem())
			field := mixed.Elem().Field(i)
			if typ.Field(i).Name == "CNPGBackupReadiness" {
				continue
			}
			field.Set(reflect.New(field.Type().Elem()))
			if mixed.Interface().(interface{ ValidCNPG() error }).ValidCNPG() == nil {
				t.Fatalf("mixed readiness accepted with %s", typ.Field(i).Name)
			}
		}
	}
}
