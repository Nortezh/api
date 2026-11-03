package api

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func rotationFixture() (DeployerCommandCNPGBackupDelete, CNPGBackupRotationPlan) {
	m := cnpgBackupDeleteCommand()
	m.Rotation = &CNPGBackupRotation{}
	p := CNPGBackupRotationPlan{Format: 1, ProfileRevision: 3, ConfigurationHash: strings.Repeat("a", 64), ObservedAt: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)}
	prefix := "base/" + m.Material.ServerName + "/"
	object := func(key string) CNPGBackupRotationObject {
		return CNPGBackupRotationObject{Key: key, SizeBytes: 128, ETag: "opaque-etag"}
	}
	p.SelectedObjects = []CNPGBackupRotationObject{object(prefix + "base/" + m.Material.BarmanBackupID + "/data.tar.gz"), object(prefix + "base/" + m.Material.BarmanBackupID + "/backup.info")}
	for n := 1; n <= 7; n++ {
		b := CNPGBackupRotationBackup{Backup: m.Backup, Material: m.Material, ArchiveBinding: m.ArchiveBinding}
		b.Backup.OperationID = fmt.Sprintf("%08x-2222-4222-8222-%012x", n, n)
		b.Backup.BackupUID = fmt.Sprintf("%08x-3333-4333-8333-%012x", n, n)
		b.Material.BarmanBackupID = fmt.Sprintf("202610%02dT120000", 15+n)
		p.ProtectedBackups = append(p.ProtectedBackups, b)
		p.ProtectedObjects = append(p.ProtectedObjects, object(prefix+"base/"+b.Material.BarmanBackupID+"/data.tar.gz"), object(prefix+"base/"+b.Material.BarmanBackupID+"/backup.info"))
	}
	m.Rotation.Replacement = p.ProtectedBackups[6]
	p.ProtectedObjects = append(p.ProtectedObjects, object(prefix+"wals/0000000100000000/000000010000000000000004.gz"))
	return m, p
}

func rotationCheckpoint(t *testing.T, m DeployerCommandCNPGBackupDelete) *CNPGBackupRotationCheckpoint {
	t.Helper()
	hash, err := m.RotationPlanSHA256()
	if err != nil {
		t.Fatal(err)
	}
	return &CNPGBackupRotationCheckpoint{JobName: "cnpg-rotation-" + strings.ToLower(m.OperationID), JobUID: "a2222222-2222-4222-8222-222222222222", PlanSHA256: hash, WorkloadSHA256: strings.Repeat("b", 64)}
}

func rotationResult(t *testing.T, status CNPGBackupRotationStatus) DeployerSetResultCNPGBackupDelete {
	t.Helper()
	m, p := rotationFixture()
	r := DeployerSetResultCNPGBackupDelete{DeployerCommandCNPGBackupDelete: m, Status: status}
	switch status {
	case CNPGBackupRotationPlanned:
		r.Plan = &p
	case CNPGBackupRotationBlocked:
		r.FailureCode = CNPGFailureBackupDelete
	default:
		r.Rotation.Plan = &p
		checkpoint := rotationCheckpoint(t, r.DeployerCommandCNPGBackupDelete)
		if status == CNPGBackupRotationCheckpointed {
			r.Checkpoint = checkpoint
		} else {
			r.Rotation.Checkpoint = checkpoint
		}
	}
	if status == CNPGBackupRotationCompleted || status == CNPGBackupRotationReconciled {
		r.Reconciliation = &CNPGWALReconciliation{Status: CNPGWALUnknown, CheckedAt: p.ObservedAt, ErrorCode: CNPGWALClassificationUnavailable}
	}
	if status == CNPGBackupRotationCompleted {
		r.Verification = &CNPGBackupRotationVerification{CheckedAt: p.ObservedAt, OldestAbsent: true, ProtectedIntact: true, TargetScopeVerified: true}
	}
	if status == CNPGBackupRotationReconciled {
		r.Rotation.ReconcileOnly = true
	}
	return r
}

func TestCNPGBackupRotationCommandAndLegacyBoundary(t *testing.T) {
	legacy := cnpgBackupDeleteCommand()
	if err := legacy.Valid(); err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(legacy)
	if strings.Contains(string(wire), `"rotation"`) {
		t.Fatal("legacy wire changed")
	}
	m, _ := rotationFixture()
	if err := m.Valid(); err != nil {
		t.Fatal(err)
	}
	if m.DeployerCommandCNPGIdentity.Valid() != nil {
		t.Fatal("rotation changed the shared identity version")
	}
	for name, change := range map[string]func(*DeployerCommandCNPGBackupDelete){
		"unsupported version": func(m *DeployerCommandCNPGBackupDelete) { m.Version = 2 },
		"last backup":         func(m *DeployerCommandCNPGBackupDelete) { m.ConfirmLastBackup = true },
		"foreign replacement": func(m *DeployerCommandCNPGBackupDelete) { m.Rotation.Replacement.Backup.ID++ },
		"foreign project":     func(m *DeployerCommandCNPGBackupDelete) { m.Rotation.Replacement.Backup.ProjectID++ },
		"changed create": func(m *DeployerCommandCNPGBackupDelete) {
			m.Rotation.Replacement.Backup.CreateOperationID = "a2222222-2222-4222-8222-222222222222"
		},
		"changed source": func(m *DeployerCommandCNPGBackupDelete) {
			m.Rotation.Replacement.Backup.SourceClusterUID = "a2222222-2222-4222-8222-222222222222"
		},
		"reused backup uid": func(m *DeployerCommandCNPGBackupDelete) {
			m.Rotation.Replacement.Backup.BackupUID = strings.ToUpper(m.Backup.BackupUID)
		},
		"reused operation": func(m *DeployerCommandCNPGBackupDelete) {
			m.Rotation.Replacement.Backup.OperationID = strings.ToUpper(m.Backup.OperationID)
		},
		"reused native id": func(m *DeployerCommandCNPGBackupDelete) {
			m.Rotation.Replacement.Material.BarmanBackupID = m.Material.BarmanBackupID
		},
		"archive drift": func(m *DeployerCommandCNPGBackupDelete) {
			m.Rotation.Replacement.ArchiveBinding.EndpointURL = "https://other.test"
		},
		"image drift": func(m *DeployerCommandCNPGBackupDelete) {
			m.Rotation.Replacement.Material.Image = "registry.test/other@sha256:" + strings.Repeat("c", 64)
		},
		"checkpoint without plan": func(m *DeployerCommandCNPGBackupDelete) { m.Rotation.Checkpoint = &CNPGBackupRotationCheckpoint{} },
		"premature reconcile":     func(m *DeployerCommandCNPGBackupDelete) { m.Rotation.ReconcileOnly = true },
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := rotationFixture()
			change(&m)
			if m.Valid() == nil {
				t.Fatal("invalid rotation accepted")
			}
		})
	}
}

func TestCNPGBackupRotationPlanScopeAndManifest(t *testing.T) {
	for name, change := range map[string]func(*CNPGBackupRotationPlan){
		"format":              func(p *CNPGBackupRotationPlan) { p.Format = 2 },
		"profile revision":    func(p *CNPGBackupRotationPlan) { p.ProfileRevision = 0 },
		"profile hash":        func(p *CNPGBackupRotationPlan) { p.ConfigurationHash = strings.Repeat("A", 64) },
		"unobserved":          func(p *CNPGBackupRotationPlan) { p.ObservedAt = time.Time{} },
		"missing survivor":    func(p *CNPGBackupRotationPlan) { p.ProtectedBackups = p.ProtectedBackups[:6] },
		"replacement missing": func(p *CNPGBackupRotationPlan) { p.ProtectedBackups[6] = p.ProtectedBackups[0] },
		"replacement changed": func(p *CNPGBackupRotationPlan) { p.ProtectedBackups[6].Material.EndLSN = "0/5000000" },
		"duplicate operation": func(p *CNPGBackupRotationPlan) {
			p.ProtectedBackups[1].Backup.OperationID = strings.ToUpper(p.ProtectedBackups[0].Backup.OperationID)
		},
		"duplicate native uid": func(p *CNPGBackupRotationPlan) {
			p.ProtectedBackups[1].Backup.BackupUID = strings.ToUpper(p.ProtectedBackups[0].Backup.BackupUID)
		},
		"duplicate native id": func(p *CNPGBackupRotationPlan) {
			p.ProtectedBackups[1].Material.BarmanBackupID = p.ProtectedBackups[0].Material.BarmanBackupID
		},
		"future survivor":        func(p *CNPGBackupRotationPlan) { p.ProtectedBackups[0].Material.BarmanBackupID = "20261030T120000" },
		"missing target info":    func(p *CNPGBackupRotationPlan) { p.SelectedObjects = p.SelectedObjects[:1] },
		"target directory":       func(p *CNPGBackupRotationPlan) { p.SelectedObjects[0].Key += "/" },
		"target wildcard":        func(p *CNPGBackupRotationPlan) { p.SelectedObjects[0].Key += "*" },
		"target url":             func(p *CNPGBackupRotationPlan) { p.SelectedObjects[0].Key = "s3://bucket/" + p.SelectedObjects[0].Key },
		"target traversal":       func(p *CNPGBackupRotationPlan) { p.SelectedObjects[0].Key += "/../escape" },
		"encoded traversal":      func(p *CNPGBackupRotationPlan) { p.SelectedObjects[0].Key += "%2f.." },
		"control key":            func(p *CNPGBackupRotationPlan) { p.SelectedObjects[0].Key += "\n" },
		"duplicate target":       func(p *CNPGBackupRotationPlan) { p.SelectedObjects = append(p.SelectedObjects, p.SelectedObjects[0]) },
		"wal delete":             func(p *CNPGBackupRotationPlan) { p.SelectedObjects[0] = p.ProtectedObjects[len(p.ProtectedObjects)-1] },
		"survivor delete":        func(p *CNPGBackupRotationPlan) { p.SelectedObjects[0] = p.ProtectedObjects[0] },
		"foreign delete":         func(p *CNPGBackupRotationPlan) { p.SelectedObjects[0].Key = "foreign/base/20261015T120000/data.tar.gz" },
		"negative bytes":         func(p *CNPGBackupRotationPlan) { p.SelectedObjects[0].SizeBytes = -1 },
		"missing etag":           func(p *CNPGBackupRotationPlan) { p.SelectedObjects[0].ETag = "" },
		"control etag":           func(p *CNPGBackupRotationPlan) { p.SelectedObjects[0].ETag = "ok\r\nsecret" },
		"missing protected info": func(p *CNPGBackupRotationPlan) { p.ProtectedObjects = p.ProtectedObjects[2:] },
		"duplicate protected object": func(p *CNPGBackupRotationPlan) {
			p.ProtectedObjects = append(p.ProtectedObjects, p.ProtectedObjects[0])
		},
		"target protected overlap": func(p *CNPGBackupRotationPlan) { p.ProtectedObjects = append(p.ProtectedObjects, p.SelectedObjects[0]) },
		"missing wal":              func(p *CNPGBackupRotationPlan) { p.ProtectedObjects = p.ProtectedObjects[:len(p.ProtectedObjects)-1] },
		"foreign protected object": func(p *CNPGBackupRotationPlan) {
			p.ProtectedObjects = append(p.ProtectedObjects, CNPGBackupRotationObject{Key: "foreign/wals/wal", ETag: "etag"})
		},
	} {
		t.Run(name, func(t *testing.T) {
			m, p := rotationFixture()
			change(&p)
			m.Rotation.Plan = &p
			if m.Valid() == nil {
				t.Fatal("unsafe plan shape accepted")
			}
		})
	}
}

func TestCNPGBackupRotationPlanHashAndCheckpoint(t *testing.T) {
	m, p := rotationFixture()
	m.Rotation.Plan = &p
	hash, err := m.RotationPlanSHA256()
	if err != nil || len(hash) != 64 {
		t.Fatal(hash, err)
	}
	checkpoint := rotationCheckpoint(t, m)
	m.Rotation.Checkpoint = checkpoint
	if err := m.Valid(); err != nil {
		t.Fatal(err)
	}
	if again, _ := m.RotationPlanSHA256(); again != hash {
		t.Fatal("checkpoint changed plan hash")
	}
	m.Rotation.ReconcileOnly = true
	if err := m.Valid(); err != nil {
		t.Fatal(err)
	}
	if again, _ := m.RotationPlanSHA256(); again != hash {
		t.Fatal("read-only mode changed original plan hash")
	}
	wire, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var decoded DeployerCommandCNPGBackupDelete
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if again, err := decoded.RotationPlanSHA256(); err != nil || again != hash || decoded.Valid() != nil {
		t.Fatal("wire changed digest", again, err)
	}
	m.Rotation.Plan.SelectedObjects[0].SizeBytes++
	if m.Valid() == nil {
		t.Fatal("mutated manifest accepted under old checkpoint")
	}
	if again, err := m.RotationPlanSHA256(); err != nil || again == hash {
		t.Fatal("manifest not bound to hash", err)
	}
	for name, change := range map[string]func(*CNPGBackupRotationCheckpoint){
		"job name":      func(c *CNPGBackupRotationCheckpoint) { c.JobName = "other-job" },
		"job uid":       func(c *CNPGBackupRotationCheckpoint) { c.JobUID = "" },
		"zero uid":      func(c *CNPGBackupRotationCheckpoint) { c.JobUID = "00000000-0000-0000-0000-000000000000" },
		"plan hash":     func(c *CNPGBackupRotationCheckpoint) { c.PlanSHA256 = strings.Repeat("c", 64) },
		"workload hash": func(c *CNPGBackupRotationCheckpoint) { c.WorkloadSHA256 = "" },
	} {
		t.Run(name, func(t *testing.T) {
			m, p := rotationFixture()
			m.Rotation.Plan = &p
			m.Rotation.Checkpoint = rotationCheckpoint(t, m)
			change(m.Rotation.Checkpoint)
			if m.Valid() == nil {
				t.Fatal("unbound checkpoint accepted")
			}
		})
	}
	if _, err := cnpgBackupDeleteCommand().RotationPlanSHA256(); err == nil {
		t.Fatal("legacy/no-plan hash accepted")
	}
}

func TestCNPGBackupRotationResultPhasesAndEnvelopes(t *testing.T) {
	for _, status := range []CNPGBackupRotationStatus{CNPGBackupRotationPlanned, CNPGBackupRotationCheckpointed, CNPGBackupRotationRunning, CNPGBackupRotationCompleted, CNPGBackupRotationBlocked, CNPGBackupRotationReconciled} {
		t.Run(string(status), func(t *testing.T) {
			r := rotationResult(t, status)
			for _, item := range []any{&DeployerCommandItem{CNPGBackupDelete: &r.DeployerCommandCNPGBackupDelete}, &DeployerSetResultItem{CNPGBackupDelete: &r}} {
				wire, err := json.Marshal(item)
				if err != nil {
					t.Fatal(err)
				}
				decoded := reflect.New(reflect.TypeOf(item).Elem()).Interface()
				if err := json.Unmarshal(wire, decoded); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(item, decoded) || decoded.(interface{ ValidCNPG() error }).ValidCNPG() != nil {
					t.Fatal("phase identity/evidence lost", string(wire))
				}
				for _, key := range []string{"password", "secretAccessKey", "accessKeyId", "token", "credentials", "failureMessage", "commandError", "argv"} {
					if strings.Contains(string(wire), `"`+key+`"`) {
						t.Fatal("credential/raw error field in wire", key)
					}
				}
				for n := 0; n < reflect.TypeOf(item).Elem().NumField(); n++ {
					if reflect.TypeOf(item).Elem().Field(n).Name == "CNPGBackupDelete" {
						continue
					}
					mixed := reflect.New(reflect.TypeOf(item).Elem())
					mixed.Elem().Set(reflect.ValueOf(item).Elem())
					field := mixed.Elem().Field(n)
					field.Set(reflect.New(field.Type().Elem()))
					if mixed.Interface().(interface{ ValidCNPG() error }).ValidCNPG() == nil {
						t.Fatal("mixed rotation accepted")
					}
				}
			}
		})
	}
	legacy := DeployerSetResultCNPGBackupDelete{DeployerCommandCNPGBackupDelete: cnpgBackupDeleteCommand(), Status: CNPGBackupRotationBlocked, FailureCode: CNPGFailureBackupDelete}
	if legacy.Valid() == nil {
		t.Fatal("GET-only v1 produced an execution result")
	}
}

func TestCNPGBackupRotationResultRejectsPrematureOrUnsafeCompletion(t *testing.T) {
	for name, change := range map[string]func(*DeployerSetResultCNPGBackupDelete){
		"unknown status":             func(r *DeployerSetResultCNPGBackupDelete) { r.Status = "unknown" },
		"exit zero alone":            func(r *DeployerSetResultCNPGBackupDelete) { r.Verification = nil },
		"no acknowledged checkpoint": func(r *DeployerSetResultCNPGBackupDelete) { r.Rotation.Checkpoint = nil },
		"target remains":             func(r *DeployerSetResultCNPGBackupDelete) { r.Verification.OldestAbsent = false },
		"protected data missing":     func(r *DeployerSetResultCNPGBackupDelete) { r.Verification.ProtectedIntact = false },
		"unconfined prior executor":  func(r *DeployerSetResultCNPGBackupDelete) { r.Verification.TargetScopeVerified = false },
		"unobserved verification":    func(r *DeployerSetResultCNPGBackupDelete) { r.Verification.CheckedAt = time.Time{} },
		"no reconciliation":          func(r *DeployerSetResultCNPGBackupDelete) { r.Reconciliation = nil },
		"raw failure":                func(r *DeployerSetResultCNPGBackupDelete) { r.FailureCode = "password=secret" },
		"unexpected plan":            func(r *DeployerSetResultCNPGBackupDelete) { r.Plan = r.Rotation.Plan },
		"unexpected checkpoint":      func(r *DeployerSetResultCNPGBackupDelete) { r.Checkpoint = r.Rotation.Checkpoint },
		"reconcile mode":             func(r *DeployerSetResultCNPGBackupDelete) { r.Rotation.ReconcileOnly = true },
	} {
		t.Run(name, func(t *testing.T) {
			r := rotationResult(t, CNPGBackupRotationCompleted)
			change(&r)
			if r.Valid() == nil {
				t.Fatal("invalid completion accepted")
			}
		})
	}
	for _, status := range []CNPGBackupRotationStatus{CNPGBackupRotationPlanned, CNPGBackupRotationCheckpointed, CNPGBackupRotationRunning, CNPGBackupRotationBlocked} {
		r := rotationResult(t, status)
		r.Verification = &CNPGBackupRotationVerification{CheckedAt: time.Now(), OldestAbsent: true, ProtectedIntact: true, TargetScopeVerified: true}
		if r.Valid() == nil {
			t.Fatal("noncompleted phase advertised completion", status)
		}
	}
	r := rotationResult(t, CNPGBackupRotationCheckpointed)
	r.Rotation.Plan = nil
	if r.Valid() == nil {
		t.Fatal("checkpoint before persisted plan accepted")
	}
	r = rotationResult(t, CNPGBackupRotationRunning)
	r.Rotation.Checkpoint = nil
	if r.Valid() == nil {
		t.Fatal("running before UID acknowledgement accepted")
	}
	r = rotationResult(t, CNPGBackupRotationPlanned)
	r.Rotation.Plan = r.Plan
	if r.Valid() == nil {
		t.Fatal("plan replacement disguised as proposal")
	}
	r = rotationResult(t, CNPGBackupRotationBlocked)
	r.FailureCode = ""
	if r.Valid() == nil {
		t.Fatal("blocked result lacked sanitized reason")
	}
}

func TestCNPGBackupRotationWALObservations(t *testing.T) {
	for _, status := range []CNPGWALReconciliationStatus{CNPGWALUnknown, CNPGWALPending, CNPGWALClean} {
		r := rotationResult(t, CNPGBackupRotationCompleted)
		w := r.Reconciliation
		w.Status = status
		switch status {
		case CNPGWALPending:
			w.KnownKeys = []string{rotationArchivePrefix(r.DeployerCommandCNPGBackupDelete) + "wals/0000000100000000/000000010000000000000003.gz"}
			w.ErrorCode = CNPGWALCleanupFailed
		case CNPGWALClean:
			zero := int64(0)
			w.KnownBytes = &zero
			w.ErrorCode = ""
		}
		if err := r.Valid(); err != nil {
			t.Fatal("cleanup state incorrectly blocked verified rotation", status, err)
		}
	}
	for name, change := range map[string]func(*CNPGWALReconciliation){
		"missing timestamp": func(w *CNPGWALReconciliation) { w.CheckedAt = time.Time{} },
		"no unknown reason": func(w *CNPGWALReconciliation) { w.ErrorCode = "" },
		"raw error":         func(w *CNPGWALReconciliation) { w.ErrorCode = "token=secret" },
		"negative bytes":    func(w *CNPGWALReconciliation) { n := int64(-1); w.KnownBytes = &n },
		"unknown status":    func(w *CNPGWALReconciliation) { w.Status = "failed" },
		"false clean":       func(w *CNPGWALReconciliation) { w.Status = CNPGWALClean; w.ErrorCode = "" },
		"empty pending":     func(w *CNPGWALReconciliation) { w.Status = CNPGWALPending },
		"foreign key":       func(w *CNPGWALReconciliation) { w.KnownKeys = []string{"foreign/wals/wal"} },
		"base relabeled as cleanup": func(w *CNPGWALReconciliation) {
			m, _ := rotationFixture()
			w.KnownKeys = []string{rotationArchivePrefix(m) + "base/" + m.Material.BarmanBackupID + "/backup.info"}
		},
		"duplicate keys": func(w *CNPGWALReconciliation) {
			m, _ := rotationFixture()
			key := rotationArchivePrefix(m) + "wals/wal"
			w.KnownKeys = []string{key, key}
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := rotationResult(t, CNPGBackupRotationCompleted)
			change(r.Reconciliation)
			if r.Valid() == nil {
				t.Fatal("invalid WAL observation accepted")
			}
		})
	}
	r := rotationResult(t, CNPGBackupRotationReconciled)
	r.Reconciliation = nil
	if r.Valid() == nil {
		t.Fatal("read-only reconciliation without observation accepted")
	}
	r = rotationResult(t, CNPGBackupRotationReconciled)
	r.Rotation.ReconcileOnly = false
	if r.Valid() == nil {
		t.Fatal("cleanup response on executable command accepted")
	}
}
