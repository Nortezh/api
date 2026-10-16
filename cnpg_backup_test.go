package api

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func cnpgBackupCommand() *DeployerCommandCNPGBackup {
	return &DeployerCommandCNPGBackup{DeployerCommandCNPGIdentity: cnpgIdentity(),
		CreateOperationID: "b1111111-1111-4111-8111-111111111111"}
}

func cnpgBackupResult() DeployerSetResultCNPGBackup {
	command := *cnpgBackupCommand()
	command.SourceClusterUID = "c1111111-1111-4111-8111-111111111111"
	command.BackupUID = "d1111111-1111-4111-8111-111111111111"
	return DeployerSetResultCNPGBackup{DeployerCommandCNPGBackup: command, Status: CNPGBackupCompleted,
		Material: &CNPGBackupMaterial{BarmanBackupID: "20261015T120000", MajorVersion: 17,
			Image: "registry.test/postgres:17@sha256:" + strings.Repeat("a", 64), StorageClass: "hostpath",
			ObjectStoreName: "backup-store", ObjectStoreUID: "e1111111-1111-4111-8111-111111111111",
			ServerName: "source-cluster", EndWAL: "000000010000000000000004", EndLSN: "0/4000028"}}
}

func TestCNPGBackupWireRoundTrip(t *testing.T) {
	result := cnpgBackupResult()
	running := result
	running.Status, running.Material = CNPGBackupRunning, nil
	failed := running
	failed.Status, failed.FailureCode = CNPGBackupFailed, CNPGFailureBackup
	for _, item := range []any{
		&DeployerCommandItem{CNPGBackup: cnpgBackupCommand()},
		&DeployerCommandItem{CNPGBackup: &result.DeployerCommandCNPGBackup},
		&DeployerSetResultItem{CNPGBackup: &running},
		&DeployerSetResultItem{CNPGBackup: &result},
		&DeployerSetResultItem{CNPGBackup: &failed},
	} {
		wire, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"password", "secretId", "secretRevision", "endpointURL", "destinationPath", "failureMessage", "commandError", "manifest", "provider", "databaseCreate", "databaseDelete"} {
			if strings.Contains(string(wire), `"`+forbidden+`"`) {
				t.Fatal("backup wire contains forbidden field", forbidden)
			}
		}
		decoded := reflect.New(reflect.TypeOf(item).Elem()).Interface()
		if err := json.Unmarshal(wire, decoded); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(item, decoded) || decoded.(interface{ ValidCNPG() error }).ValidCNPG() != nil {
			t.Fatal("backup envelope lost valid identity/evidence")
		}
	}
	command := cnpgBackupCommand()
	command.ID, command.ProjectID = 90071992547409931, 90071992547409933
	wire, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	var decoded DeployerCommandCNPGBackup
	if err := json.Unmarshal(wire, &decoded); err != nil || decoded != *command {
		t.Fatal("internal Go wire lost large integer identity", err)
	}
}

func TestCNPGBackupCommandValidation(t *testing.T) {
	if cnpgBackupCommand().Valid() != nil {
		t.Fatal("initial backup command rejected")
	}
	for _, change := range []func(*DeployerCommandCNPGBackup){
		func(m *DeployerCommandCNPGBackup) { m.Version = 0 },
		func(m *DeployerCommandCNPGBackup) { m.Version = 2 },
		func(m *DeployerCommandCNPGBackup) { m.ID = 0 },
		func(m *DeployerCommandCNPGBackup) { m.ProjectID = 0 },
		func(m *DeployerCommandCNPGBackup) { m.OperationID = "invalid" },
		func(m *DeployerCommandCNPGBackup) { m.CreateOperationID = "" },
		func(m *DeployerCommandCNPGBackup) { m.CreateOperationID = "00000000-0000-0000-0000-000000000000" },
		func(m *DeployerCommandCNPGBackup) { m.CreateOperationID = strings.ToUpper(m.OperationID) },
		func(m *DeployerCommandCNPGBackup) { m.SourceClusterUID = "c1111111-1111-4111-8111-111111111111" },
		func(m *DeployerCommandCNPGBackup) { m.BackupUID = "d1111111-1111-4111-8111-111111111111" },
		func(m *DeployerCommandCNPGBackup) { m.SourceClusterUID, m.BackupUID = "invalid", "invalid" },
		func(m *DeployerCommandCNPGBackup) {
			m.SourceClusterUID, m.BackupUID = "00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000"
		},
	} {
		command := cnpgBackupCommand()
		change(command)
		if command.Valid() == nil {
			t.Fatal("invalid backup identity/checkpoint accepted")
		}
	}
}

func TestCNPGBackupResultValidation(t *testing.T) {
	for _, change := range []func(*DeployerSetResultCNPGBackup){
		func(m *DeployerSetResultCNPGBackup) { m.BackupUID = "" },
		func(m *DeployerSetResultCNPGBackup) { m.SourceClusterUID = "" },
		func(m *DeployerSetResultCNPGBackup) { m.Status = "pending" },
		func(m *DeployerSetResultCNPGBackup) { m.Status = "unknown" },
		func(m *DeployerSetResultCNPGBackup) { m.Material = nil },
		func(m *DeployerSetResultCNPGBackup) { m.FailureCode = CNPGFailureBackup },
		func(m *DeployerSetResultCNPGBackup) { m.Status = CNPGBackupRunning },
		func(m *DeployerSetResultCNPGBackup) { m.Status, m.FailureCode = CNPGBackupFailed, CNPGFailureBackup },
		func(m *DeployerSetResultCNPGBackup) { m.Status, m.Material = CNPGBackupFailed, nil },
		func(m *DeployerSetResultCNPGBackup) {
			m.Status, m.Material, m.FailureCode = CNPGBackupFailed, nil, CNPGFailureBackup
			m.SourceClusterUID, m.BackupUID = "", ""
		},
		func(m *DeployerSetResultCNPGBackup) {
			m.Status, m.Material, m.FailureCode = CNPGBackupFailed, nil, CNPGFailureReconcile
		},
		func(m *DeployerSetResultCNPGBackup) {
			m.Status, m.Material, m.FailureCode = CNPGBackupFailed, nil, "raw upstream details"
		},
		func(m *DeployerSetResultCNPGBackup) {
			m.Status, m.Material, m.FailureCode = CNPGBackupRunning, nil, CNPGFailureBackup
		},
	} {
		result := cnpgBackupResult()
		change(&result)
		if result.Valid() == nil {
			t.Fatal("contradictory backup outcome accepted")
		}
	}
}

func TestCNPGBackupMaterialValidation(t *testing.T) {
	for _, change := range []func(*CNPGBackupMaterial){
		func(m *CNPGBackupMaterial) { m.BarmanBackupID = "" },
		func(m *CNPGBackupMaterial) { m.BarmanBackupID = "latest" },
		func(m *CNPGBackupMaterial) { m.BarmanBackupID = "20260230T120000" },
		func(m *CNPGBackupMaterial) { m.BarmanBackupID = "../20261015T120000" },
		func(m *CNPGBackupMaterial) { m.MajorVersion = 0 },
		func(m *CNPGBackupMaterial) { m.Image = "registry.test/postgres:latest" },
		func(m *CNPGBackupMaterial) { m.Image = "registry.test/postgres@sha256:" + strings.Repeat("g", 64) },
		func(m *CNPGBackupMaterial) {
			m.Image = "https://private@registry/postgres@sha256:" + strings.Repeat("a", 64)
		},
		func(m *CNPGBackupMaterial) { m.StorageClass = "" },
		func(m *CNPGBackupMaterial) { m.StorageClass = strings.Repeat("a", 64) },
		func(m *CNPGBackupMaterial) { m.ObjectStoreName = "s3://private/bucket" },
		func(m *CNPGBackupMaterial) { m.ObjectStoreName = "backup..store" },
		func(m *CNPGBackupMaterial) { m.ObjectStoreUID = "" },
		func(m *CNPGBackupMaterial) { m.ServerName = "../source" },
		func(m *CNPGBackupMaterial) { m.ServerName = strings.Repeat("a.", 127) + "b" },
		func(m *CNPGBackupMaterial) { m.EndWAL = "" },
		func(m *CNPGBackupMaterial) { m.EndWAL = strings.Repeat("0", 24) },
		func(m *CNPGBackupMaterial) { m.EndWAL = "00000001000000000000000G" },
		func(m *CNPGBackupMaterial) { m.EndLSN = "" },
		func(m *CNPGBackupMaterial) { m.EndLSN = "FFFFFFFFF/1" },
		func(m *CNPGBackupMaterial) { m.EndLSN = "raw upstream details" },
	} {
		result := cnpgBackupResult()
		change(result.Material)
		if result.Valid() == nil {
			t.Fatal("incomplete/unsafe recovery material accepted")
		}
	}
	result := cnpgBackupResult()
	result.Material.StorageClass = "hostpath.csi.test"
	if result.Valid() != nil {
		t.Fatal("valid DNS subdomain storage class rejected")
	}
}

func TestCNPGBackupValidationDoesNotEchoNativeDetails(t *testing.T) {
	const private = "DO_NOT_ECHO_NATIVE_DETAILS"
	for _, material := range []bool{false, true} {
		result := cnpgBackupResult()
		if material {
			result.Material.EndLSN = private
		} else {
			result.Status, result.Material, result.FailureCode = CNPGBackupFailed, nil, CNPGFailureCode(private)
		}
		if err := result.Valid(); err == nil || strings.Contains(err.Error(), private) {
			t.Fatal("validation accepted or echoed raw native details")
		}
	}
}

func TestCNPGBackupEnvelopeExclusivity(t *testing.T) {
	for _, item := range []any{&DeployerCommandItem{}, &DeployerSetResultItem{}} {
		typ := reflect.TypeOf(item).Elem()
		for i := 0; i < typ.NumField(); i++ {
			m := reflect.New(typ)
			field := m.Elem().Field(i)
			field.Set(reflect.New(field.Type().Elem()))
			if typ == reflect.TypeOf(DeployerCommandItem{}) {
				m.Elem().FieldByName("CNPGBackup").Set(reflect.ValueOf(cnpgBackupCommand()))
			} else {
				result := cnpgBackupResult()
				m.Elem().FieldByName("CNPGBackup").Set(reflect.ValueOf(&result))
			}
			want := typ.Field(i).Name == "CNPGBackup"
			if (m.Interface().(interface{ ValidCNPG() error }).ValidCNPG() == nil) != want {
				t.Fatal("backup mixed envelope/exclusivity bypass", typ, typ.Field(i).Name)
			}
		}
	}
}
