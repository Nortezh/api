package api

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func cnpgIdentity() DeployerCommandCNPGIdentity {
	return DeployerCommandCNPGIdentity{Version: 1, ID: 1, ProjectID: 2, OperationID: "a1111111-1111-4111-8111-111111111111"}
}

func cnpgDelete() *DeployerCommandCNPGDelete {
	identity := cnpgIdentity()
	identity.Version = CNPGDeleteVersion
	return &DeployerCommandCNPGDelete{DeployerCommandCNPGIdentity: identity, Phase: CNPGDeleteResources, SecretID: 3, SecretRevision: 1, CreateOperationID: "b1111111-1111-4111-8111-111111111111"}
}

func TestCNPGWireRoundTrip(t *testing.T) {
	identity := cnpgIdentity()
	for _, value := range []any{
		&DeployerCommandItem{CNPGCreate: &DeployerCommandCNPGCreate{DeployerCommandCNPGIdentity: identity, Name: "db", Username: "app", StorageSize: 1024, SecretID: 3, SecretRevision: 1}},
		&DeployerCommandItem{CNPGDelete: cnpgDelete()},
		&DeployerSetResultItem{CNPGCreate: &DeployerSetResultCNPGCreate{DeployerCommandCNPGIdentity: identity, Success: true, Host: "db.example.test", Port: 5432, QueryReady: true, TLSReady: true}},
		&DeployerSetResultItem{CNPGDelete: &DeployerSetResultCNPGDelete{DeployerCommandCNPGIdentity: cnpgDelete().DeployerCommandCNPGIdentity, Phase: CNPGDeleteResources, ResourceAbsent: true}},
	} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"password", "provider", "databaseCreate", "databaseDelete", "failureMessage", "storageClass", "image"} {
			if strings.Contains(string(data), `"`+forbidden+`"`) {
				t.Fatalf("unexpected %s in %s", forbidden, data)
			}
		}
		target := reflect.New(reflect.TypeOf(value).Elem()).Interface()
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(value, target) {
			t.Fatalf("round trip changed %s", data)
		}
		if err := target.(interface{ ValidCNPG() error }).ValidCNPG(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCNPGIdentityAndCreateValidation(t *testing.T) {
	for _, change := range []func(*DeployerCommandCNPGIdentity){
		func(m *DeployerCommandCNPGIdentity) { m.Version = 0 },
		func(m *DeployerCommandCNPGIdentity) { m.Version = 2 },
		func(m *DeployerCommandCNPGIdentity) { m.ID = 0 },
		func(m *DeployerCommandCNPGIdentity) { m.ProjectID = -1 },
		func(m *DeployerCommandCNPGIdentity) { m.OperationID = "" },
		func(m *DeployerCommandCNPGIdentity) { m.OperationID = "not-a-uuid" },
		func(m *DeployerCommandCNPGIdentity) { m.OperationID = "00000000-0000-0000-0000-000000000000" },
	} {
		m := cnpgIdentity()
		change(&m)
		if m.Valid() == nil {
			t.Fatalf("accepted invalid identity: %+v", m)
		}
	}
	for _, change := range []func(*DeployerCommandCNPGCreate){
		func(m *DeployerCommandCNPGCreate) { m.Name = "" },
		func(m *DeployerCommandCNPGCreate) { m.Username = "" },
		func(m *DeployerCommandCNPGCreate) { m.StorageSize = 0 },
		func(m *DeployerCommandCNPGCreate) { m.SecretID = 0 },
		func(m *DeployerCommandCNPGCreate) { m.SecretRevision = 0 },
	} {
		m := DeployerCommandCNPGCreate{DeployerCommandCNPGIdentity: cnpgIdentity(), Name: "db", Username: "app", StorageSize: 1024, SecretID: 3, SecretRevision: 1}
		change(&m)
		if m.Valid() == nil {
			t.Fatalf("accepted invalid create: %+v", m)
		}
	}
}

func TestCNPGReadinessAndAbsenceValidation(t *testing.T) {
	ready := DeployerSetResultCNPGCreate{DeployerCommandCNPGIdentity: cnpgIdentity(), Success: true, Host: "db.example.test", Port: 5432, QueryReady: true, TLSReady: true}
	for _, change := range []func(*DeployerSetResultCNPGCreate){
		func(m *DeployerSetResultCNPGCreate) { m.QueryReady = false },
		func(m *DeployerSetResultCNPGCreate) { m.TLSReady = false },
		func(m *DeployerSetResultCNPGCreate) { m.Port = 65536 },
		func(m *DeployerSetResultCNPGCreate) { m.Host = "" },
		func(m *DeployerSetResultCNPGCreate) { m.FailureCode = CNPGFailureQuery },
		func(m *DeployerSetResultCNPGCreate) { m.Success = false; m.FailureCode = CNPGFailureTLS },
	} {
		m := ready
		change(&m)
		if m.Valid() == nil {
			t.Fatalf("accepted contradictory readiness: %+v", m)
		}
	}
	for _, code := range []CNPGFailureCode{CNPGFailureReconcile, CNPGFailureQuery, CNPGFailureTLS, "", "raw password", CNPGFailureCleanup} {
		m := DeployerSetResultCNPGCreate{DeployerCommandCNPGIdentity: cnpgIdentity(), FailureCode: code}
		want := code == CNPGFailureReconcile || code == CNPGFailureQuery || code == CNPGFailureTLS
		if (m.Valid() == nil) != want {
			t.Fatalf("unexpected create failure validation: %q", code)
		}
	}
	for _, tc := range []struct {
		absent bool
		code   CNPGFailureCode
		valid  bool
	}{
		{true, "", true}, {false, CNPGFailureCleanup, true}, {false, "", false}, {true, CNPGFailureCleanup, false}, {false, "raw error", false},
	} {
		m := DeployerSetResultCNPGDelete{DeployerCommandCNPGIdentity: cnpgDelete().DeployerCommandCNPGIdentity, Phase: CNPGDeleteResources, ResourceAbsent: tc.absent, FailureCode: tc.code}
		if (m.Valid() == nil) != tc.valid {
			t.Fatalf("unexpected delete validation: %+v", tc)
		}
	}
}

func TestCNPGEnvelopeExclusivity(t *testing.T) {
	identity := cnpgDelete().DeployerCommandCNPGIdentity
	for _, value := range []any{&DeployerCommandItem{}, &DeployerSetResultItem{}} {
		typ := reflect.TypeOf(value).Elem()
		for i := 0; i < typ.NumField(); i++ {
			m := reflect.New(typ)
			field := m.Elem().Field(i)
			field.Set(reflect.New(field.Type().Elem()))
			deleteField := m.Elem().FieldByName("CNPGDelete")
			if typ == reflect.TypeOf(DeployerCommandItem{}) {
				deleteField.Set(reflect.ValueOf(cnpgDelete()))
			} else {
				deleteField.Set(reflect.ValueOf(&DeployerSetResultCNPGDelete{DeployerCommandCNPGIdentity: identity, Phase: CNPGDeleteResources, ResourceAbsent: true}))
			}
			want := typ.Field(i).Name == "CNPGDelete"
			if (m.Interface().(interface{ ValidCNPG() error }).ValidCNPG() == nil) != want {
				t.Fatalf("unexpected exclusivity for %s.%s", typ, typ.Field(i).Name)
			}
		}
	}
	var command *DeployerCommandItem
	var result *DeployerSetResultItem
	if command.ValidCNPG() == nil || result.ValidCNPG() == nil {
		t.Fatal("nil envelope accepted")
	}
	if (&DeployerCommandItem{}).ValidCNPG() == nil || (&DeployerSetResultItem{}).ValidCNPG() == nil {
		t.Fatal("empty envelope accepted")
	}
}

func TestCNPGDoesNotChangeLegacyEnvelopeJSON(t *testing.T) {
	for _, tc := range []struct {
		value any
		want  string
	}{
		{&DeployerCommandItem{DatabaseDelete: &DeployerCommandDatabaseMetadata{ID: 1, ProjectID: 2, Name: "db", Type: DatabaseTypePostgres}}, `{"databaseDelete":{"id":1,"projectId":2,"name":"db","type":"postgres"}}`},
		{&DeployerSetResultItem{DatabaseCreate: &DeployerSetResultItemDatabaseCreate{ID: 1, Success: true, Host: "db.test", Port: 5432}}, `{"databaseCreate":{"id":1,"success":true,"host":"db.test","port":5432}}`},
	} {
		got, err := json.Marshal(tc.value)
		if err != nil || string(got) != tc.want {
			t.Fatalf("legacy JSON changed: %s, %v", got, err)
		}
	}
}
