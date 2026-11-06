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
	CNPGDeleteVersion   = 3
	CNPGDeleteResources = "resources"
	CNPGDeleteFinalize  = "finalize"
)

// Phase separates archive/resource retirement from release after Secret sync.
// Older archive-unaware commands/results remain non-executable.
type DeployerCommandCNPGDelete struct {
	DeployerCommandCNPGIdentity
	Phase             string             `json:"phase"`
	SecretID          int64              `json:"secretId"`
	SecretRevision    int64              `json:"secretRevision"`
	CreateOperationID string             `json:"createOperationId"`
	Receipt           *CNPGDeleteReceipt `json:"receipt,omitempty"`
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
	v.Must(cnpgBackupUUID(m.CreateOperationID) && !strings.EqualFold(m.CreateOperationID, m.OperationID), "CNPG create identity invalid")
	if m.Phase == CNPGDeleteResources {
		v.Must(m.Receipt == nil, "resource cleanup cannot adopt a final receipt")
	} else {
		v.Must(m.Receipt != nil && m.Receipt.Valid() == nil, "finalize requires the accepted archive receipt")
	}
	return WrapValidate(v)
}

const (
	CNPGDeleteScope    = "scope"
	CNPGDeleteWriters  = "writers"
	CNPGDeleteArchives = "archives"
)

// Receipt attests the supported native lifecycle, not provider fencing or physical GC.
// Raw logs, object keys and credentials stay out of the wire checkpoint.
type CNPGDeleteReceipt struct {
	InventoryUID       string `json:"inventoryUid"`
	ClusterUID         string `json:"clusterUid"`
	ArchiveBindingHash string `json:"archiveBindingHash"`
	WriterStopped      bool   `json:"writerStopped"`
	ArchiveAbsent      bool   `json:"archiveAbsent"`
}

func (m CNPGDeleteReceipt) Valid() error {
	v := validator.New()
	v.Must(cnpgBackupUUID(m.InventoryUID) && cnpgBackupUUID(m.ClusterUID) && !strings.EqualFold(m.InventoryUID, m.ClusterUID), "CNPG retirement identity invalid")
	v.Must(len(m.ArchiveBindingHash) == 64 && cnpgReadinessConfigurationHash.MatchString(m.ArchiveBindingHash), "CNPG archive binding hash invalid")
	v.Must(m.WriterStopped && m.ArchiveAbsent, "CNPG committed archive cleanup not confirmed")
	return WrapValidate(v)
}

type DeployerSetResultCNPGDelete struct {
	DeployerCommandCNPGIdentity
	Phase             string             `json:"phase"`
	ResourceAbsent    bool               `json:"resourceAbsent"`
	CreateOperationID string             `json:"createOperationId"`
	SecretID          int64              `json:"secretId"`
	SecretRevision    int64              `json:"secretRevision"`
	Receipt           *CNPGDeleteReceipt `json:"receipt,omitempty"`
	FailureCode       CNPGFailureCode    `json:"failureCode,omitempty"`
	FailureStage      string             `json:"failureStage,omitempty"`
}

func (m DeployerSetResultCNPGDelete) Valid() error {
	c := DeployerCommandCNPGDelete{DeployerCommandCNPGIdentity: m.DeployerCommandCNPGIdentity, Phase: m.Phase,
		CreateOperationID: m.CreateOperationID, SecretID: m.SecretID, SecretRevision: m.SecretRevision}
	if m.Phase == CNPGDeleteFinalize {
		c.Receipt = m.Receipt
	}
	if err := c.Valid(); err != nil {
		return err
	}
	v := validator.New()
	if m.ResourceAbsent {
		v.Must(m.Receipt != nil && m.Receipt.Valid() == nil && m.FailureCode == "" && m.FailureStage == "", "CNPG archive/resource cleanup not confirmed")
	} else {
		v.Must(m.FailureCode == CNPGFailureCleanup, "CNPG cleanup failure code invalid")
		if m.Phase == CNPGDeleteResources {
			v.Must(m.Receipt == nil, "failed resources cannot certify archive cleanup")
			v.Must(m.FailureStage == CNPGDeleteScope || m.FailureStage == CNPGDeleteWriters || m.FailureStage == CNPGDeleteArchives || m.FailureStage == CNPGDeleteResources, "CNPG cleanup stage invalid")
		} else {
			v.Must(m.FailureStage == CNPGDeleteFinalize, "CNPG finalize failure stage invalid")
		}
	}
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
	if m.CNPGBackupRetention != nil {
		return m.CNPGBackupRetention.Valid()
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
	if m.CNPGBackupRetention != nil {
		return m.CNPGBackupRetention.Valid()
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
		cnpg = cnpg || name == "CNPGCreate" || name == "CNPGDelete" || name == "CNPGBackup" || name == "CNPGRestore" || name == "CNPGBackupReadiness" || name == "CNPGBackupRetention"
	}
	return count == 1 && cnpg
}
