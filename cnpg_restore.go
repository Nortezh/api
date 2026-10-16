package api

import "github.com/moonrhythm/validator"

// The embedded create identity belongs to the NEW target, never the source.
// Backup carries the selected source/native checkpoint; no latest-backup fallback.
type DeployerCommandCNPGRestore struct {
	DeployerCommandCNPGCreate
	RestoreOperationID string                    `json:"restoreOperationId"`
	Backup             DeployerCommandCNPGBackup `json:"backup"`
	Material           CNPGBackupMaterial        `json:"material"`
}

func (m DeployerCommandCNPGRestore) Valid() error {
	if err := m.DeployerCommandCNPGCreate.Valid(); err != nil {
		return err
	}
	if err := m.Backup.Valid(); err != nil {
		return err
	}
	if err := m.Material.Valid(); err != nil {
		return err
	}
	v := validator.New()
	v.Must(cnpgBackupUUID(m.RestoreOperationID) && m.RestoreOperationID != m.OperationID && m.RestoreOperationID != m.Backup.OperationID, "CNPG restore operation invalid")
	v.Must(m.ID != m.Backup.ID && m.ProjectID == m.Backup.ProjectID && m.OperationID != m.Backup.CreateOperationID, "CNPG restore target scope invalid")
	v.Must(cnpgBackupUUID(m.Backup.SourceClusterUID) && cnpgBackupUUID(m.Backup.BackupUID), "CNPG selected backup checkpoint required")
	return WrapValidate(v)
}

const CNPGFailureRestore CNPGFailureCode = "restore_failed"

// Uncertain recovery remains active; terminal failure requires native recovery Job evidence.
type DeployerSetResultCNPGRestore struct {
	DeployerCommandCNPGRestore
	Status           CNPGBackupStatus `json:"status"`
	Host             string           `json:"host,omitempty"`
	Port             int              `json:"port,omitempty"`
	QueryReady       bool             `json:"queryReady"`
	TLSReady         bool             `json:"tlsReady"`
	FailureCode      CNPGFailureCode  `json:"failureCode,omitempty"`
	TargetClusterUID string           `json:"targetClusterUid,omitempty"`
	RecoveryJobUID   string           `json:"recoveryJobUid,omitempty"`
}

func (m DeployerSetResultCNPGRestore) Valid() error {
	if err := m.DeployerCommandCNPGRestore.Valid(); err != nil {
		return err
	}
	v := validator.New()
	if m.Status != CNPGBackupFailed {
		v.Must(m.FailureCode == "" && m.TargetClusterUID == "" && m.RecoveryJobUID == "", "unexpected restore failure evidence")
	}
	switch m.Status {
	case CNPGBackupFailed:
		v.Must(m.Host == "" && m.Port == 0 && !m.QueryReady && !m.TLSReady, "failed restore cannot advertise readiness")
		v.Must(m.FailureCode == CNPGFailureRestore && cnpgBackupUUID(m.TargetClusterUID) && cnpgBackupUUID(m.RecoveryJobUID), "native restore failure evidence required")
	case CNPGBackupRunning:
		v.Must(m.Host == "" && m.Port == 0 && !m.QueryReady && !m.TLSReady, "running restore cannot advertise readiness")
	case CNPGBackupCompleted:
		v.Must(m.Host != "" && m.Port > 0 && m.Port <= 65535 && m.QueryReady && m.TLSReady, "restore readiness evidence required")
	default:
		v.Must(false, "CNPG restore observation invalid")
	}
	return WrapValidate(v)
}
