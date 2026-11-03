package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/moonrhythm/validator"
)

// No Plan: observe/propose only. Plan without Checkpoint: create/observe only a
// suspended Job. Only Backend's persisted UID acknowledgement can permit resume;
// consumers must authenticate location and match the entire immutable intent.
// This initial contract grants no WAL deletion or automatic repair/retry phase.
type CNPGBackupRotation struct {
	Replacement   CNPGBackupRotationBackup      `json:"replacement"`
	Plan          *CNPGBackupRotationPlan       `json:"plan,omitempty"`
	Checkpoint    *CNPGBackupRotationCheckpoint `json:"checkpoint,omitempty"`
	ReconcileOnly bool                          `json:"reconcileOnly,omitempty"`
}

type CNPGBackupRotationBackup struct {
	Backup         DeployerCommandCNPGBackup `json:"backup"`
	Material       CNPGBackupMaterial        `json:"material"`
	ArchiveBinding CNPGBackupArchiveBinding  `json:"archiveBinding"`
}

func (b CNPGBackupRotationBackup) valid(m DeployerCommandCNPGBackupDelete) error {
	if err := b.Backup.Valid(); err != nil {
		return err
	}
	if err := b.ArchiveBinding.Valid(&b.Material); err != nil {
		return err
	}
	v := validator.New()
	v.Must(b.Backup.ID == m.ID && b.Backup.ProjectID == m.ProjectID &&
		strings.EqualFold(b.Backup.CreateOperationID, m.Backup.CreateOperationID) &&
		strings.EqualFold(b.Backup.SourceClusterUID, m.Backup.SourceClusterUID) && cnpgBackupUUID(b.Backup.BackupUID), "CNPG rotation backup scope invalid")
	v.Must(!strings.EqualFold(b.Backup.OperationID, m.OperationID) && !strings.EqualFold(b.Backup.OperationID, m.Backup.OperationID) &&
		!strings.EqualFold(b.Backup.BackupUID, m.Backup.BackupUID), "CNPG rotation backup identity reused")
	v.Must(b.Material.ServerName == m.Material.ServerName && b.Material.ObjectStoreName == m.Material.ObjectStoreName &&
		strings.EqualFold(b.Material.ObjectStoreUID, m.Material.ObjectStoreUID) && b.Material.MajorVersion == m.Material.MajorVersion &&
		b.Material.Image == m.Material.Image && b.Material.StorageClass == m.Material.StorageClass &&
		b.ArchiveBinding.EndpointURL == m.ArchiveBinding.EndpointURL && b.ArchiveBinding.DestinationPath == m.ArchiveBinding.DestinationPath,
		"CNPG rotation backup archive/profile mismatch")
	v.Must(b.Material.BarmanBackupID > m.Material.BarmanBackupID, "CNPG rotation survivor must follow oldest")
	return WrapValidate(v)
}

func (r CNPGBackupRotation) valid(m DeployerCommandCNPGBackupDelete) error {
	if err := r.Replacement.valid(m); err != nil {
		return err
	}
	if r.Plan != nil {
		if err := r.Plan.valid(m); err != nil {
			return err
		}
	}
	v := validator.New()
	v.Must(!r.ReconcileOnly || r.Checkpoint != nil, "CNPG WAL reconciliation requires original execution checkpoint")
	if err := WrapValidate(v); err != nil {
		return err
	}
	if r.Checkpoint != nil {
		return r.Checkpoint.valid(m)
	}
	return nil
}

// Exact raw object keys, not prefixes, URLs or CLI output. ETag is an opaque
// identity observation, not a content checksum or a conditional-delete fence.
type CNPGBackupRotationObject struct {
	Key       string `json:"key"`
	SizeBytes int64  `json:"sizeBytes"`
	ETag      string `json:"etag"`
}

var cnpgRotationKey = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)

func validRotationKey(key string) bool {
	// Initial stock-key profile only; never normalize/URL-decode a delete key.
	return len(key) > 0 && len(key) <= 1024 && cnpgRotationKey.MatchString(key) &&
		!strings.HasPrefix(key, "/") && !strings.HasSuffix(key, "/") && path.Clean(key) == key
}

func (o CNPGBackupRotationObject) valid() bool {
	return validRotationKey(o.Key) && o.SizeBytes >= 0 && strings.TrimSpace(o.ETag) != "" && len(o.ETag) <= 256 &&
		strings.IndexFunc(o.ETag, unicode.IsControl) < 0
}

// The complete manifest is persisted before irreversible execution. The Worker
// must independently prove pagination, supported full-backup files, required WAL
// coverage/timelines and Restore dependencies; Valid checks only wire structure.
// SelectedObjects is the ENTIRE possible Delete authority; WAL is never in it.
type CNPGBackupRotationPlan struct {
	Format            int                        `json:"format"`
	ProfileRevision   int64                      `json:"profileRevision"`
	ConfigurationHash string                     `json:"configurationHash"`
	ObservedAt        time.Time                  `json:"observedAt"`
	SelectedObjects   []CNPGBackupRotationObject `json:"selectedObjects"`
	ProtectedBackups  []CNPGBackupRotationBackup `json:"protectedBackups"`
	ProtectedObjects  []CNPGBackupRotationObject `json:"protectedObjects"`
}

func rotationArchivePrefix(m DeployerCommandCNPGBackupDelete) string {
	// The enclosing command's ArchiveBinding.Valid has already checked this URL.
	d, _ := url.Parse(m.ArchiveBinding.DestinationPath)
	return path.Join(strings.TrimPrefix(d.Path, "/"), m.Material.ServerName) + "/"
}

func (p CNPGBackupRotationPlan) valid(m DeployerCommandCNPGBackupDelete) error {
	v := validator.New()
	v.Must(p.Format == 1 && p.ProfileRevision > 0 && cnpgReadinessConfigurationHash.MatchString(p.ConfigurationHash) && !p.ObservedAt.IsZero(), "CNPG rotation plan provenance required")
	v.Must(len(p.ProtectedBackups) == 7, "CNPG rotation requires seven protected backups")
	prefix := rotationArchivePrefix(m)
	target := prefix + "base/" + m.Material.BarmanBackupID + "/"
	objects := make(map[string]bool)
	for _, o := range p.SelectedObjects {
		v.Must(o.valid() && strings.HasPrefix(o.Key, target) && !objects[o.Key], "CNPG rotation selected object invalid/duplicated")
		objects[o.Key] = true
	}
	v.Must(len(p.SelectedObjects) >= 2 && objects[target+"backup.info"], "CNPG rotation target base/metadata manifest incomplete")
	operations, nativeUIDs, nativeIDs := make(map[string]bool), make(map[string]bool), make(map[string]bool)
	protectedPrefixes := make(map[string]bool)
	replacementFound := false
	for _, b := range p.ProtectedBackups {
		if err := b.valid(m); err != nil {
			return err
		}
		op, uid := strings.ToLower(b.Backup.OperationID), strings.ToLower(b.Backup.BackupUID)
		v.Must(!operations[op] && !nativeUIDs[uid] && !nativeIDs[b.Material.BarmanBackupID], "CNPG rotation protected backup duplicated")
		operations[op], nativeUIDs[uid], nativeIDs[b.Material.BarmanBackupID] = true, true, true
		protectedPrefixes[prefix+"base/"+b.Material.BarmanBackupID+"/"] = true
		if strings.EqualFold(b.Backup.OperationID, m.Rotation.Replacement.Backup.OperationID) {
			v.Must(b == m.Rotation.Replacement, "CNPG rotation replacement material changed")
			replacementFound = true
		}
		v.Must(b.Material.BarmanBackupID <= m.Rotation.Replacement.Material.BarmanBackupID, "CNPG rotation replacement is not newest")
	}
	v.Must(replacementFound, "CNPG rotation replacement missing from protected backups")
	protected := make(map[string]bool)
	walFound := false
	for _, o := range p.ProtectedObjects {
		allowed := strings.HasPrefix(o.Key, prefix+"wals/")
		walFound = walFound || allowed
		for base := range protectedPrefixes {
			allowed = allowed || strings.HasPrefix(o.Key, base)
		}
		v.Must(o.valid() && allowed && !objects[o.Key] && !protected[o.Key], "CNPG rotation protected object invalid/overlapping/duplicated")
		protected[o.Key] = true
	}
	for base := range protectedPrefixes {
		files := 0
		for key := range protected {
			if strings.HasPrefix(key, base) {
				files++
			}
		}
		v.Must(protected[base+"backup.info"] && files >= 2, "CNPG rotation protected base/metadata manifest incomplete")
	}
	v.Must(walFound, "CNPG rotation protected WAL manifest required")
	return WrapValidate(v)
}

// Hash the typed JSON intent+plan, excluding Checkpoint and the read-only
// ReconcileOnly mode. Array order is immutable; never reorder to authorize a Job.
func (m DeployerCommandCNPGBackupDelete) RotationPlanSHA256() (string, error) {
	if m.Rotation == nil || m.Rotation.Plan == nil {
		return "", fmt.Errorf("CNPG rotation plan required")
	}
	r := *m.Rotation
	r.Checkpoint = nil
	r.ReconcileOnly = false
	m.Rotation = &r
	if err := m.Valid(); err != nil {
		return "", err
	}
	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

// Persist exact UID before resume. WorkloadSHA256 covers reviewed non-secret
// workload/scope configuration, never credential values. Neither a UID nor a
// matching hash proves effective provider authority or prior-effect safety.
type CNPGBackupRotationCheckpoint struct {
	JobName        string `json:"jobName"`
	JobUID         string `json:"jobUid"`
	PlanSHA256     string `json:"planSha256"`
	WorkloadSHA256 string `json:"workloadSha256"`
}

func (c CNPGBackupRotationCheckpoint) valid(m DeployerCommandCNPGBackupDelete) error {
	hash, err := m.RotationPlanSHA256()
	if err != nil {
		return err
	}
	v := validator.New()
	v.Must(c.JobName == "cnpg-rotation-"+strings.ToLower(m.OperationID) && cnpgBackupUUID(c.JobUID), "CNPG rotation Job identity invalid")
	v.Must(c.PlanSHA256 == hash && cnpgReadinessConfigurationHash.MatchString(c.WorkloadSHA256), "CNPG rotation checkpoint binding invalid")
	return WrapValidate(v)
}

// Shape only: these attestations must come from independent readback/authority
// checks on the Worker, never a CLI exit code, missing Job or credential expiry.
type CNPGBackupRotationVerification struct {
	CheckedAt           time.Time `json:"checkedAt"`
	OldestAbsent        bool      `json:"oldestAbsent"`
	ProtectedIntact     bool      `json:"protectedIntact"`
	TargetScopeVerified bool      `json:"targetScopeVerified"`
}

type CNPGWALReconciliationStatus string

const (
	CNPGWALPending CNPGWALReconciliationStatus = "pending"
	CNPGWALUnknown CNPGWALReconciliationStatus = "unknown"
	CNPGWALClean   CNPGWALReconciliationStatus = "clean"
)

type CNPGWALReconciliationError string

const (
	CNPGWALInventoryUnavailable      CNPGWALReconciliationError = "inventory_unavailable"
	CNPGWALClassificationUnavailable CNPGWALReconciliationError = "classification_unavailable"
	CNPGWALCleanupFailed             CNPGWALReconciliationError = "cleanup_failed"
)

// KnownKeys are observations, not permission to delete or a claim of obsolescence.
// Unknown bytes remain nil, not a fabricated zero. Clean is one complete checked
// observation, not permanent absence or a promise of next-day collection.
type CNPGWALReconciliation struct {
	Status     CNPGWALReconciliationStatus `json:"status"`
	CheckedAt  time.Time                   `json:"checkedAt"`
	KnownKeys  []string                    `json:"knownKeys,omitempty"`
	KnownBytes *int64                      `json:"knownBytes,omitempty"`
	ErrorCode  CNPGWALReconciliationError  `json:"errorCode,omitempty"`
}

func (r CNPGWALReconciliation) valid(m DeployerCommandCNPGBackupDelete) error {
	v := validator.New()
	v.Must(!r.CheckedAt.IsZero() && (r.KnownBytes == nil || *r.KnownBytes >= 0), "CNPG WAL observation invalid")
	keys := make(map[string]bool)
	for _, key := range r.KnownKeys {
		v.Must(validRotationKey(key) && strings.HasPrefix(key, rotationArchivePrefix(m)+"wals/") && !keys[key], "CNPG WAL observation key invalid/duplicated")
		keys[key] = true
	}
	errValid := r.ErrorCode == CNPGWALInventoryUnavailable || r.ErrorCode == CNPGWALClassificationUnavailable || r.ErrorCode == CNPGWALCleanupFailed
	switch r.Status {
	case CNPGWALPending:
		v.Must(len(r.KnownKeys) > 0 && (r.ErrorCode == "" || errValid), "CNPG pending WAL observation invalid")
	case CNPGWALUnknown:
		v.Must(errValid, "CNPG unknown WAL observation requires sanitized error")
	case CNPGWALClean:
		v.Must(len(r.KnownKeys) == 0 && r.KnownBytes != nil && *r.KnownBytes == 0 && r.ErrorCode == "", "CNPG clean WAL observation invalid")
	default:
		v.Must(false, "CNPG WAL reconciliation status invalid")
	}
	return WrapValidate(v)
}

type CNPGBackupRotationStatus string

const (
	CNPGBackupRotationPlanned      CNPGBackupRotationStatus = "planned"
	CNPGBackupRotationCheckpointed CNPGBackupRotationStatus = "checkpointed"
	CNPGBackupRotationRunning      CNPGBackupRotationStatus = "running"
	CNPGBackupRotationCompleted    CNPGBackupRotationStatus = "completed"
	CNPGBackupRotationReconciled   CNPGBackupRotationStatus = "reconciled"
	CNPGBackupRotationBlocked      CNPGBackupRotationStatus = "blocked"
	CNPGFailureBackupDelete        CNPGFailureCode          = "backup_delete_failed"
)

// A proposal/observation echoes the accepted command. Backend must persist and
// compare original identity/plan/UID before acknowledging any new phase. Blocked
// is action-required under the same guard, not a terminal retry/unlock signal.
type DeployerSetResultCNPGBackupDelete struct {
	DeployerCommandCNPGBackupDelete
	Status         CNPGBackupRotationStatus        `json:"status"`
	Plan           *CNPGBackupRotationPlan         `json:"plan,omitempty"`
	Checkpoint     *CNPGBackupRotationCheckpoint   `json:"checkpoint,omitempty"`
	Verification   *CNPGBackupRotationVerification `json:"verification,omitempty"`
	Reconciliation *CNPGWALReconciliation          `json:"reconciliation,omitempty"`
	FailureCode    CNPGFailureCode                 `json:"failureCode,omitempty"`
}

func (m DeployerSetResultCNPGBackupDelete) Valid() error {
	if err := m.DeployerCommandCNPGBackupDelete.Valid(); err != nil {
		return err
	}
	v := validator.New()
	v.Must(m.Rotation != nil, "GET-only CNPG delete has no execution result")
	if err := WrapValidate(v); err != nil {
		return err
	}
	v.Must(m.Status == CNPGBackupRotationBlocked || m.FailureCode == "", "CNPG rotation unexpected failure code")
	v.Must(m.Status == CNPGBackupRotationPlanned || m.Plan == nil, "CNPG rotation unexpected plan proposal")
	v.Must(m.Status == CNPGBackupRotationCheckpointed || m.Checkpoint == nil, "CNPG rotation unexpected checkpoint proposal")
	v.Must((m.Status == CNPGBackupRotationReconciled) == m.Rotation.ReconcileOnly, "CNPG rotation reconciliation mode mismatch")
	v.Must(m.Status == CNPGBackupRotationCompleted || m.Verification == nil, "CNPG rotation premature completion evidence")
	v.Must(m.Status == CNPGBackupRotationCompleted || m.Status == CNPGBackupRotationReconciled || m.Reconciliation == nil, "CNPG rotation unexpected WAL observation")
	switch m.Status {
	case CNPGBackupRotationPlanned:
		v.Must(m.Rotation.Plan == nil && m.Rotation.Checkpoint == nil && m.Plan != nil, "CNPG rotation plan proposal phase invalid")
		if m.Plan != nil {
			if err := m.Plan.valid(m.DeployerCommandCNPGBackupDelete); err != nil {
				return err
			}
		}
	case CNPGBackupRotationCheckpointed:
		v.Must(m.Rotation.Plan != nil && m.Rotation.Checkpoint == nil && m.Checkpoint != nil, "CNPG rotation checkpoint proposal phase invalid")
		if m.Checkpoint != nil {
			if err := m.Checkpoint.valid(m.DeployerCommandCNPGBackupDelete); err != nil {
				return err
			}
		}
	case CNPGBackupRotationRunning:
		v.Must(m.Rotation.Checkpoint != nil, "CNPG rotation acknowledged checkpoint required")
	case CNPGBackupRotationCompleted:
		v.Must(m.Rotation.Checkpoint != nil && m.Verification != nil && m.Reconciliation != nil, "CNPG rotation independent verification required")
		if m.Verification != nil {
			b := m.Verification
			v.Must(!b.CheckedAt.IsZero() && b.OldestAbsent && b.ProtectedIntact && b.TargetScopeVerified, "CNPG rotation safety not verified")
		}
		if m.Reconciliation != nil {
			if err := m.Reconciliation.valid(m.DeployerCommandCNPGBackupDelete); err != nil {
				return err
			}
		}
	case CNPGBackupRotationReconciled:
		// Backend must authorize this only after persisted rotation completion;
		// it updates cleanup observations, never terminal history or capacity.
		v.Must(m.Reconciliation != nil, "CNPG WAL reconciliation observation required")
		if m.Reconciliation != nil {
			if err := m.Reconciliation.valid(m.DeployerCommandCNPGBackupDelete); err != nil {
				return err
			}
		}
	case CNPGBackupRotationBlocked:
		v.Must(m.FailureCode == CNPGFailureBackupDelete, "CNPG blocking rotation requires sanitized failure")
	default:
		v.Must(false, "CNPG rotation result status invalid")
	}
	return WrapValidate(v)
}
