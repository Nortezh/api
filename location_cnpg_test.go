package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDatabaseCNPGCapabilityDefaultsUnavailable(t *testing.T) {
	var old LocationFeatures
	if err := json.Unmarshal([]byte(`{"database":{}}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Database == nil || old.DatabaseCNPG != nil {
		t.Fatal("generic database must not imply CNPG availability")
	}
	wire, err := json.Marshal(old)
	if err != nil || strings.Contains(string(wire), "databaseCnpg") {
		t.Fatal("old location JSON changed", err)
	}
	var configured LocationFeatures
	if err := json.Unmarshal([]byte(`{"databaseCnpg":{"configured":true}}`), &configured); err != nil {
		t.Fatal(err)
	}
	if configured.DatabaseCNPG == nil || !configured.DatabaseCNPG.Configured || configured.DatabaseCNPG.CreateAvailable {
		t.Fatal("configuration enabled creation without explicit permission")
	}
}
