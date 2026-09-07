package api

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDeploymentSecretEnvJSON(t *testing.T) {
	ref := DeployerCommandDeploymentSecretEnv{EnvName: "TOKEN", SecretID: 42}
	got, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"envName":"TOKEN","secretId":42}`; string(got) != want {
		t.Fatalf("reference: got %s, want %s", got, want)
	}

	for _, tt := range []struct {
		name string
		spec DeployerCommandDeploymentDeploySpec
		want bool
	}{
		{"omitted", DeployerCommandDeploymentDeploySpec{}, false},
		{"present", DeployerCommandDeploymentDeploySpec{SecretEnvs: []DeployerCommandDeploymentSecretEnv{ref}}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.spec)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(got, &fields); err != nil {
				t.Fatal(err)
			}
			refs, ok := fields["secretEnvs"]
			if ok != tt.want {
				t.Fatalf("secretEnvs presence: got %t, want %t", ok, tt.want)
			}
			if ok && string(refs) != `[{"envName":"TOKEN","secretId":42}]` {
				t.Fatalf("secretEnvs: got %s", refs)
			}
		})
	}
}

func TestSecretCommandAndResultJSON(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		json   string
		target func() any
	}{
		{
			"upsert command",
			&DeployerCommandItem{SecretUpsert: &DeployerCommandSecretUpsert{SecretID: 1, ProjectID: 2, Name: "TOKEN", Revision: 3, Value: "secret"}},
			`{"secretUpsert":{"secretId":1,"projectId":2,"name":"TOKEN","revision":3,"value":"secret"}}`,
			func() any { return &DeployerCommandItem{} },
		},
		{
			"delete command",
			&DeployerCommandItem{SecretDelete: &DeployerCommandSecretDelete{SecretID: 1, ProjectID: 2, Name: "TOKEN", Revision: 3}},
			`{"secretDelete":{"secretId":1,"projectId":2,"name":"TOKEN","revision":3}}`,
			func() any { return &DeployerCommandItem{} },
		},
		{
			"upsert result",
			&DeployerSetResultItem{SecretUpsert: &DeployerSetResultItemSecret{SecretID: 1, Revision: 3, Success: true}},
			`{"secretUpsert":{"secretId":1,"revision":3,"success":true}}`,
			func() any { return &DeployerSetResultItem{} },
		},
		{
			"delete result",
			&DeployerSetResultItem{SecretDelete: &DeployerSetResultItemSecret{SecretID: 1, Revision: 3, Success: false}},
			`{"secretDelete":{"secretId":1,"revision":3,"success":false}}`,
			func() any { return &DeployerSetResultItem{} },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.json {
				t.Fatalf("marshal: got %s, want %s", got, tt.json)
			}

			roundTrip := tt.target()
			if err := json.Unmarshal(got, roundTrip); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(roundTrip, tt.value) {
				t.Fatalf("unmarshal: got %#v, want %#v", roundTrip, tt.value)
			}
		})
	}
}
