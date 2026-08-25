package api

import (
	"encoding/json"
	"reflect"
	"testing"
)

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
