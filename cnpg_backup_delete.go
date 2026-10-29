package api

import (
	"fmt"
	"strings"

	"github.com/moonrhythm/validator"
)

// An accepted selected-delete intent, not permission to run a destructive command.
// The current Worker only observes this checkpoint; no physical delete result is defined.
type DeployerCommandCNPGBackupDelete struct {
	DeployerCommandCNPGIdentity
	Backup            DeployerCommandCNPGBackup `json:"backup"`
	Material          CNPGBackupMaterial        `json:"material"`
	ArchiveBinding    CNPGBackupArchiveBinding  `json:"archiveBinding"`
	ConfirmLastBackup bool                      `json:"confirmLastBackup"`
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
	return WrapValidate(v)
}
