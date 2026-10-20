package api

import (
	"regexp"
	"strings"

	"github.com/moonrhythm/validator"
)

// Read-only verification, not backup intent or permission to patch a source.
// OperationID is the source's original provisioning identity. Backend must bind
// CheckID and the expected location-owned ProfileRevision to current intent.
// Wire validation alone does not establish freshness, ownership or readiness.
type DeployerCommandCNPGBackupReadiness struct {
	DeployerCommandCNPGIdentity
	CheckID         string `json:"checkId"`
	ProfileRevision int64  `json:"profileRevision"`
}

func (m DeployerCommandCNPGBackupReadiness) Valid() error {
	if err := m.DeployerCommandCNPGIdentity.Valid(); err != nil {
		return err
	}
	v := validator.New()
	v.Must(cnpgBackupUUID(m.CheckID) && !strings.EqualFold(m.CheckID, m.OperationID), "CNPG readiness check identity invalid")
	v.Must(m.ProfileRevision > 0, "CNPG readiness profile revision invalid")
	return WrapValidate(v)
}

// Internal observed provenance, never a customer response or storage credential.
// A Worker must verify the exact source/bootstrap identity, supported plugin,
// compatible ObjectStore/access and successful WAL archiving before sending it.
// ConfigurationHash binds the verified profile/ObjectStore configuration; it
// must not be computed from credential values or treated as an access token.
type CNPGBackupReadinessEvidence struct {
	SourceClusterUID  string `json:"sourceClusterUid"`
	ObjectStoreName   string `json:"objectStoreName"`
	ObjectStoreUID    string `json:"objectStoreUid"`
	ServerName        string `json:"serverName"`
	ConfigurationHash string `json:"configurationHash"`
}

var cnpgReadinessConfigurationHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

func (m CNPGBackupReadinessEvidence) Valid() error {
	v := validator.New()
	v.Must(cnpgBackupUUID(m.SourceClusterUID), "CNPG readiness source identity invalid")
	v.Must(cnpgBackupDNSName(m.ObjectStoreName) && cnpgBackupUUID(m.ObjectStoreUID), "CNPG readiness ObjectStore identity invalid")
	v.Must(cnpgBackupDNSName(m.ServerName), "CNPG readiness server identity invalid")
	v.Must(len(m.ConfigurationHash) == 64 && cnpgReadinessConfigurationHash.MatchString(m.ConfigurationHash), "CNPG readiness configuration hash invalid")
	return WrapValidate(v)
}

const CNPGFailureBackupReadiness CNPGFailureCode = "backup_unavailable"

// Unavailable (including unverified prerequisites) disables new admission only;
// it is never a failed Backup receipt or evidence to release an active guard.
// Backend must reject stale checks/profile revisions before storing evidence.
// No worker timestamp can extend Backend-owned validity or authorize backfill.
type DeployerSetResultCNPGBackupReadiness struct {
	DeployerCommandCNPGBackupReadiness
	Available   bool                         `json:"available"`
	Evidence    *CNPGBackupReadinessEvidence `json:"evidence,omitempty"`
	FailureCode CNPGFailureCode              `json:"failureCode,omitempty"`
}

func (m DeployerSetResultCNPGBackupReadiness) Valid() error {
	if err := m.DeployerCommandCNPGBackupReadiness.Valid(); err != nil {
		return err
	}
	v := validator.New()
	if m.Available {
		v.Must(m.Evidence != nil && m.FailureCode == "", "CNPG readiness evidence required")
		if m.Evidence != nil {
			if err := m.Evidence.Valid(); err != nil {
				return err
			}
		}
	} else {
		v.Must(m.Evidence == nil && m.FailureCode == CNPGFailureBackupReadiness, "CNPG unavailable readiness invalid")
	}
	return WrapValidate(v)
}
