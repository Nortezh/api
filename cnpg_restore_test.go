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
	for _, m := range []any{&DeployerCommandItem{CNPGRestore: &c}, &DeployerSetResultItem{CNPGRestore: &running}, &DeployerSetResultItem{CNPGRestore: &completed}} {
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
	completed.TLSReady = false
	if completed.Valid() == nil {
		t.Fatal("unverified completion accepted")
	}
	if (&DeployerCommandItem{CNPGRestore: &c, DatabaseCreate: &DeployerCommandDatabaseCreate{}}).ValidCNPG() == nil {
		t.Fatal("mixed legacy command accepted")
	}
	if (&DeployerSetResultItem{CNPGRestore: &running, SecretDelete: &DeployerSetResultItemSecret{}}).ValidCNPG() == nil {
		t.Fatal("mixed legacy result accepted")
	}
}
