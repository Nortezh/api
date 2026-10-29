package api

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func cnpgBackupDeleteCommand() DeployerCommandCNPGBackupDelete {
	r := cnpgBackupResult()
	m := DeployerCommandCNPGBackupDelete{DeployerCommandCNPGIdentity: r.DeployerCommandCNPGIdentity,
		Backup: r.DeployerCommandCNPGBackup, Material: *r.Material, ArchiveBinding: archiveBindingFixture()}
	m.OperationID = "f1111111-1111-4111-8111-111111111111"
	m.Material.ServerName = fmt.Sprintf("cnpg-%d-%d", m.ID, m.ProjectID)
	return m
}

func TestCNPGBackupDeleteCommandValidation(t *testing.T) {
	good := cnpgBackupDeleteCommand()
	if err := good.Valid(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*DeployerCommandCNPGBackupDelete){
		"version":                func(m *DeployerCommandCNPGBackupDelete) { m.Version = 2 },
		"source":                 func(m *DeployerCommandCNPGBackupDelete) { m.Backup.ID++ },
		"project":                func(m *DeployerCommandCNPGBackupDelete) { m.Backup.ProjectID++ },
		"backup operation":       func(m *DeployerCommandCNPGBackupDelete) { m.OperationID = m.Backup.OperationID },
		"case changed operation": func(m *DeployerCommandCNPGBackupDelete) { m.OperationID = strings.ToUpper(m.Backup.OperationID) },
		"create operation":       func(m *DeployerCommandCNPGBackupDelete) { m.OperationID = m.Backup.CreateOperationID },
		"checkpoint":             func(m *DeployerCommandCNPGBackupDelete) { m.Backup.BackupUID = "" },
		"binding":                func(m *DeployerCommandCNPGBackupDelete) { m.ArchiveBinding = CNPGBackupArchiveBinding{} },
		"material":               func(m *DeployerCommandCNPGBackupDelete) { m.Material.BarmanBackupID = "latest" },
		"server":                 func(m *DeployerCommandCNPGBackupDelete) { m.Material.ServerName = "foreign" },
	} {
		t.Run(name, func(t *testing.T) {
			m := good
			change(&m)
			if m.Valid() == nil {
				t.Fatal("invalid selected delete accepted")
			}
		})
	}
}

func TestCNPGBackupDeleteWireAndExclusivity(t *testing.T) {
	for _, confirm := range []bool{false, true} {
		m := cnpgBackupDeleteCommand()
		m.ConfirmLastBackup = confirm
		envelope := &DeployerCommandItem{CNPGBackupDelete: &m}
		wire, err := json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		var decoded DeployerCommandItem
		if json.Unmarshal(wire, &decoded) != nil || decoded.ValidCNPG() != nil || !reflect.DeepEqual(envelope, &decoded) {
			t.Fatal("selected identity or confirmation lost in wire")
		}
		for i := 0; i < reflect.TypeOf(decoded).NumField(); i++ {
			mixed := decoded
			field := reflect.ValueOf(&mixed).Elem().Field(i)
			if reflect.TypeOf(decoded).Field(i).Name == "CNPGBackupDelete" {
				continue
			}
			field.Set(reflect.New(field.Type().Elem()))
			if mixed.ValidCNPG() == nil {
				t.Fatal("mixed selected delete accepted")
			}
		}
	}
}
