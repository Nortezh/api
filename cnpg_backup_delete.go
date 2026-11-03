package api

import (
	"fmt"
	"strings"

	"github.com/moonrhythm/validator"
)

// Without Rotation, this is GET-only. Rotation carries the system intent and
// acknowledged plan/checkpoint state; syntax alone never authorizes execution.
type DeployerCommandCNPGBackupDelete struct {
	DeployerCommandCNPGIdentity
	Backup            DeployerCommandCNPGBackup `json:"backup"`
	Material          CNPGBackupMaterial        `json:"material"`
	ArchiveBinding    CNPGBackupArchiveBinding  `json:"archiveBinding"`
	ConfirmLastBackup bool                      `json:"confirmLastBackup"`
	Rotation          *CNPGBackupRotation       `json:"rotation,omitempty"`
}

func (m DeployerCommandCNPGBackupDelete) Valid() error {
	if err := m.DeployerCommandCNPGIdentity.Valid(); err != nil {
		return err
	}
	if err := m.Backup.Valid(); err != nil {
		return err
	}
	if err := m.ArchiveBinding.Valid(&m.Material); err != nil {
		return err
	}
	v := validator.New()
	v.Must(m.ID == m.Backup.ID && m.ProjectID == m.Backup.ProjectID, "CNPG selected delete source scope invalid")
	v.Must(!strings.EqualFold(m.OperationID, m.Backup.OperationID) && !strings.EqualFold(m.OperationID, m.Backup.CreateOperationID), "CNPG selected delete operation invalid")
	v.Must(cnpgBackupUUID(m.Backup.SourceClusterUID) && cnpgBackupUUID(m.Backup.BackupUID), "CNPG selected delete checkpoint required")
	v.Must(m.Material.ServerName == fmt.Sprintf("cnpg-%d-%d", m.ID, m.ProjectID), "CNPG selected delete archive server invalid")
	v.Must(m.Rotation == nil || !m.ConfirmLastBackup, "CNPG rotation cannot confirm last backup")
	if err := WrapValidate(v); err != nil {
		return err
	}
	if m.Rotation != nil {
		return m.Rotation.valid(m)
	}
	return nil
}
