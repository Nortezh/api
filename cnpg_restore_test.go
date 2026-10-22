package api

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func restoreCommand() DeployerCommandCNPGRestore {
	b := cnpgBackupResult()
	target := DeployerCommandCNPGCreate{DeployerCommandCNPGIdentity: cnpgIdentity(), Name: "restored", Username: "app", Database: "app", StorageSize: 1024, SecretID: 4, SecretRevision: 1}
	target.ID = 3
	target.OperationID = "f1111111-1111-4111-8111-111111111111"
	return DeployerCommandCNPGRestore{DeployerCommandCNPGCreate: target, RestoreOperationID: "91111111-1111-4111-8111-111111111111", Backup: b.DeployerCommandCNPGBackup, Material: *b.Material}
}

func TestCNPGRestoreWire(t *testing.T) {
	c := restoreCommand()
	running := DeployerSetResultCNPGRestore{DeployerCommandCNPGRestore: c, Status: CNPGBackupRunning}
	completed := running
	completed.Status = CNPGBackupCompleted
	completed.Host = "localhost"
	completed.Port = 16201
	completed.QueryReady = true
	completed.TLSReady = true
	failed := running
	failed.Status, failed.FailureCode = CNPGBackupFailed, CNPGFailureRestore
	failed.TargetClusterUID, failed.RecoveryJobUID = "71111111-1111-4111-8111-111111111111", "61111111-1111-4111-8111-111111111111"
	for _, m := range []any{&DeployerCommandItem{CNPGRestore: &c}, &DeployerSetResultItem{CNPGRestore: &running}, &DeployerSetResultItem{CNPGRestore: &completed}, &DeployerSetResultItem{CNPGRestore: &failed}} {
		if err := m.(interface{ ValidCNPG() error }).ValidCNPG(); err != nil {
			t.Fatal(err)
		}
		wire, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"password", "destinationPath", "endpointURL", "failureMessage"} {
			if strings.Contains(string(wire), `"`+field+`"`) {
				t.Fatal("credential/diagnostic wire field")
			}
		}
		decoded := reflect.New(reflect.TypeOf(m).Elem()).Interface()
		if json.Unmarshal(wire, decoded) != nil || !reflect.DeepEqual(m, decoded) {
			t.Fatal("roundtrip changed selected intent")
		}
	}
	for _, change := range []func(*DeployerCommandCNPGRestore){
		func(m *DeployerCommandCNPGRestore) { m.ID = m.Backup.ID },
		func(m *DeployerCommandCNPGRestore) { m.ProjectID++ },
		func(m *DeployerCommandCNPGRestore) { m.Backup.BackupUID = ""; m.Backup.SourceClusterUID = "" },
		func(m *DeployerCommandCNPGRestore) { m.Material.BarmanBackupID = "latest" },
		func(m *DeployerCommandCNPGRestore) { m.RestoreOperationID = m.OperationID },
	} {
		m := restoreCommand()
		change(&m)
		if m.Valid() == nil {
			t.Fatal("invalid restore accepted")
		}
	}
	for _, change := range []func(*DeployerSetResultCNPGRestore){
		func(r *DeployerSetResultCNPGRestore) { r.Status = CNPGBackupFailed },
		func(r *DeployerSetResultCNPGRestore) { r.Status = CNPGBackupStatus("unknown") },
		func(r *DeployerSetResultCNPGRestore) { r.Host, r.Port = "localhost", 16201 },
		func(r *DeployerSetResultCNPGRestore) { r.QueryReady = true },
		func(r *DeployerSetResultCNPGRestore) { r.TLSReady = true },
	} {
		invalid := running
		change(&invalid)
		if invalid.Valid() == nil {
			t.Fatal("unsupported/inconsistent running restore accepted")
		}
	}
	for _, change := range []func(*DeployerSetResultCNPGRestore){
		func(r *DeployerSetResultCNPGRestore) { r.Host = "" },
		func(r *DeployerSetResultCNPGRestore) { r.Port = 0 },
		func(r *DeployerSetResultCNPGRestore) { r.QueryReady = false },
		func(r *DeployerSetResultCNPGRestore) { r.TLSReady = false },
	} {
		invalid := completed
		change(&invalid)
		if invalid.Valid() == nil {
			t.Fatal("restore completed without full readiness evidence")
		}
	}
	for _, change := range []func(*DeployerSetResultCNPGRestore){
		func(r *DeployerSetResultCNPGRestore) { r.TargetClusterUID = "" },
		func(r *DeployerSetResultCNPGRestore) { r.RecoveryJobUID = "" },
		func(r *DeployerSetResultCNPGRestore) { r.FailureCode = CNPGFailureBackup },
		func(r *DeployerSetResultCNPGRestore) { r.TLSReady = true },
		func(r *DeployerSetResultCNPGRestore) { r.Status = CNPGBackupRunning },
	} {
		invalid := failed
		change(&invalid)
		if invalid.Valid() == nil {
			t.Fatal("invalid native failure evidence accepted")
		}
	}
	if (&DeployerCommandItem{CNPGRestore: &c, DatabaseCreate: &DeployerCommandDatabaseCreate{}}).ValidCNPG() == nil {
		t.Fatal("mixed legacy command accepted")
	}
	if (&DeployerSetResultItem{CNPGRestore: &running, SecretDelete: &DeployerSetResultItemSecret{}}).ValidCNPG() == nil {
		t.Fatal("mixed legacy result accepted")
	}
}
