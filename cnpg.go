package api

import (
	"reflect"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/moonrhythm/validator"
)

// CNPG wire types do not authorize dispatch; location authority comes from authentication.
type DeployerCommandCNPGIdentity struct {
	Version     int    `json:"version"`
	ID          int64  `json:"id"`
	ProjectID   int64  `json:"projectId"`
	OperationID string `json:"operationId"`
}

func (m DeployerCommandCNPGIdentity) Valid() error {
	v := validator.New()
	v.Must(m.Version == 1, "unsupported CNPG version")
	v.Must(m.ID > 0 && m.ProjectID > 0, "CNPG identity invalid")
	v.Must(len(m.OperationID) == 36 && govalidator.IsUUID(m.OperationID) && m.OperationID != "00000000-0000-0000-0000-000000000000", "CNPG operation invalid")
	return WrapValidate(v)
}

type DeployerCommandCNPGCreate struct {
	DeployerCommandCNPGIdentity
	Name           string            `json:"name"`
	Username       string            `json:"username"`
	Database       string            `json:"database"`
	StorageSize    int64             `json:"storageSize"` // MiB per instance
	Resources      DatabaseResources `json:"resources"`
	SecretID       int64             `json:"secretId"`
	SecretRevision int64             `json:"secretRevision"`
}

func (m DeployerCommandCNPGCreate) Valid() error {
	if err := m.DeployerCommandCNPGIdentity.Valid(); err != nil {
		return err
	}
	v := validator.New()
	v.Must(m.Name != "" && m.Username != "", "CNPG name and username required")
	v.Must(m.StorageSize > 0, "CNPG storage size invalid")
	v.Must(m.SecretID > 0 && m.SecretRevision > 0, "CNPG credential reference invalid")
	return WrapValidate(v)
}

type CNPGFailureCode string

const (
	CNPGFailureReconcile CNPGFailureCode = "reconcile_failed"
	CNPGFailureQuery     CNPGFailureCode = "query_failed"
	CNPGFailureTLS       CNPGFailureCode = "tls_failed"
	CNPGFailureCleanup   CNPGFailureCode = "cleanup_failed"
)

type DeployerSetResultCNPGCreate struct {
	DeployerCommandCNPGIdentity
	Success     bool            `json:"success"`
	Host        string          `json:"host"`
	Port        int             `json:"port"`
	QueryReady  bool            `json:"queryReady"`
	TLSReady    bool            `json:"tlsReady"`
	FailureCode CNPGFailureCode `json:"failureCode,omitempty"`
}

func (m DeployerSetResultCNPGCreate) Valid() error {
	if err := m.DeployerCommandCNPGIdentity.Valid(); err != nil {
		return err
	}
	v := validator.New()
	if m.Success {
		v.Must(m.Host != "" && m.Port > 0 && m.Port <= 65535 && m.QueryReady && m.TLSReady && m.FailureCode == "", "CNPG readiness evidence invalid")
	} else {
		v.Must(m.Host == "" && m.Port == 0 && !m.QueryReady && !m.TLSReady, "failed CNPG result cannot advertise readiness")
		v.Must(m.FailureCode == CNPGFailureReconcile || m.FailureCode == CNPGFailureQuery || m.FailureCode == CNPGFailureTLS, "CNPG failure code invalid")
	}
	return WrapValidate(v)
}

const (
	CNPGDeleteVersion   = 2
	CNPGDeleteResources = "resources"
	CNPGDeleteFinalize  = "finalize"
)

// Phase separates resource retirement from endpoint/name release after Secret sync.
// Old identity-only deletes remain non-executable.
type DeployerCommandCNPGDelete struct {
	DeployerCommandCNPGIdentity
	Phase             string `json:"phase"`
	SecretID          int64  `json:"secretId"`
	SecretRevision    int64  `json:"secretRevision"`
	CreateOperationID string `json:"createOperationId"`
}

func validCNPGDeleteIdentity(m DeployerCommandCNPGIdentity) error {
	v := validator.New()
	v.Must(m.Version == CNPGDeleteVersion, "unsupported CNPG delete version")
	if err := WrapValidate(v); err != nil {
		return err
	}
	m.Version = 1
	return m.Valid()
}

func (m DeployerCommandCNPGDelete) Valid() error {
	if err := validCNPGDeleteIdentity(m.DeployerCommandCNPGIdentity); err != nil {
		return err
	}
	v := validator.New()
	v.Must(m.Phase == CNPGDeleteResources || m.Phase == CNPGDeleteFinalize, "CNPG delete phase invalid")
	v.Must(m.SecretID > 0 && m.SecretRevision > 0, "CNPG delete credential identity invalid")
	v.Must(len(m.CreateOperationID) == 36 && govalidator.IsUUID(m.CreateOperationID) && m.CreateOperationID != "00000000-0000-0000-0000-000000000000" && !strings.EqualFold(m.CreateOperationID, m.OperationID), "CNPG create identity invalid")
	return WrapValidate(v)
}

type DeployerSetResultCNPGDelete struct {
	DeployerCommandCNPGIdentity
	Phase          string          `json:"phase"`
	ResourceAbsent bool            `json:"resourceAbsent"`
	FailureCode    CNPGFailureCode `json:"failureCode,omitempty"`
}

func (m DeployerSetResultCNPGDelete) Valid() error {
	if err := validCNPGDeleteIdentity(m.DeployerCommandCNPGIdentity); err != nil {
		return err
	}
	v := validator.New()
	v.Must(m.Phase == CNPGDeleteResources || m.Phase == CNPGDeleteFinalize, "CNPG delete phase invalid")
	v.Must((m.ResourceAbsent && m.FailureCode == "") || (!m.ResourceAbsent && m.FailureCode == CNPGFailureCleanup), "CNPG deletion evidence invalid")
	return WrapValidate(v)
}

// ValidCNPG validates wire shape only, not ownership, sync or permission to execute.
func (m *DeployerCommandItem) ValidCNPG() error {
	v := validator.New()
	v.Must(m != nil && soleCNPGMember(m), "CNPG command must be the sole operation")
	if err := WrapValidate(v); err != nil {
		return err
	}
	if m.CNPGBackupReadiness != nil {
		return m.CNPGBackupReadiness.Valid()
	}
	if m.CNPGCreate != nil {
		return m.CNPGCreate.Valid()
	}
	if m.CNPGBackup != nil {
		return m.CNPGBackup.Valid()
	}
	if m.CNPGRestore != nil {
		return m.CNPGRestore.Valid()
	}
	if m.CNPGBackupDelete != nil {
		return m.CNPGBackupDelete.Valid()
	}
	return m.CNPGDelete.Valid()
}

func (m *DeployerSetResultItem) ValidCNPG() error {
	v := validator.New()
	v.Must(m != nil && soleCNPGMember(m), "CNPG result must be the sole operation")
	if err := WrapValidate(v); err != nil {
		return err
	}
	if m.CNPGBackupReadiness != nil {
		return m.CNPGBackupReadiness.Valid()
	}
	if m.CNPGCreate != nil {
		return m.CNPGCreate.Valid()
	}
	if m.CNPGBackup != nil {
		return m.CNPGBackup.Valid()
	}
	if m.CNPGRestore != nil {
		return m.CNPGRestore.Valid()
	}
	return m.CNPGDelete.Valid()
}

func soleCNPGMember(m any) bool {
	fields := reflect.ValueOf(m).Elem()
	count := 0
	cnpg := false
	// Count every envelope member so future legacy additions cannot bypass exclusivity.
	for i := 0; i < fields.NumField(); i++ {
		if fields.Field(i).IsNil() {
			continue
		}
		count++
		name := fields.Type().Field(i).Name
		cnpg = cnpg || name == "CNPGCreate" || name == "CNPGDelete" || name == "CNPGBackup" || name == "CNPGRestore" || name == "CNPGBackupReadiness" || name == "CNPGBackupDelete"
	}
	return count == 1 && cnpg
}
