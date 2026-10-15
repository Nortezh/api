package api

import (
	"regexp"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/moonrhythm/validator"
)

// ID identifies the source; OperationID identifies this backup (not its create).
// Optional native UIDs are an observed checkpoint, never permission to adopt replacements.
// Dispatch/reconciliation must authenticate location and match current persisted intent.
type DeployerCommandCNPGBackup struct {
	DeployerCommandCNPGIdentity
	CreateOperationID string `json:"createOperationId"`
	SourceClusterUID  string `json:"sourceClusterUid,omitempty"`
	BackupUID         string `json:"backupUid,omitempty"`
}

func (m DeployerCommandCNPGBackup) Valid() error {
	if err := m.DeployerCommandCNPGIdentity.Valid(); err != nil {
		return err
	}
	v := validator.New()
	v.Must(cnpgBackupUUID(m.CreateOperationID) && !strings.EqualFold(m.CreateOperationID, m.OperationID), "CNPG source create identity invalid")
	v.Must((m.SourceClusterUID == "" && m.BackupUID == "") ||
		(cnpgBackupUUID(m.SourceClusterUID) && cnpgBackupUUID(m.BackupUID)), "CNPG backup checkpoint invalid")
	return WrapValidate(v)
}

type CNPGBackupStatus string

const (
	CNPGBackupRunning   CNPGBackupStatus = "running"
	CNPGBackupCompleted CNPGBackupStatus = "completed"
	CNPGBackupFailed    CNPGBackupStatus = "failed"
	CNPGFailureBackup   CNPGFailureCode  = "backup_failed"
)

// Internal recovery metadata, never a public backup response or storage credential.
// The worker must verify the exact stock Backup/catalog/WAL and location profile;
// a syntactically valid receipt alone proves none of those facts.
type CNPGBackupMaterial struct {
	BarmanBackupID  string `json:"barmanBackupId"`
	MajorVersion    int    `json:"majorVersion"`
	Image           string `json:"image"`
	StorageClass    string `json:"storageClass"`
	ObjectStoreName string `json:"objectStoreName"`
	ObjectStoreUID  string `json:"objectStoreUid"`
	ServerName      string `json:"serverName"`
	EndWAL          string `json:"endWal"`
	EndLSN          string `json:"endLsn"`
}

var (
	cnpgBackupDNSLabel = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	cnpgBackupImage    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]*@sha256:[a-f0-9]{64}$`)
	cnpgBackupWAL      = regexp.MustCompile(`^[0-9A-F]{24}$`)
	cnpgBackupLSN      = regexp.MustCompile(`^[0-9A-F]{1,8}/[0-9A-F]{1,8}$`)
)

func (m CNPGBackupMaterial) Valid() error {
	_, backupTimeErr := time.Parse("20060102T150405", m.BarmanBackupID)
	v := validator.New()
	v.Must(len(m.BarmanBackupID) == 15 && backupTimeErr == nil, "CNPG Barman backup identity invalid")
	v.Must(m.MajorVersion > 0, "CNPG backup major version required")
	v.Must(len(m.Image) <= 512 && cnpgBackupImage.MatchString(m.Image), "CNPG backup pinned image required")
	v.Must(cnpgBackupDNSName(m.StorageClass), "CNPG backup storage class invalid")
	v.Must(cnpgBackupDNSName(m.ObjectStoreName) && cnpgBackupUUID(m.ObjectStoreUID), "CNPG backup ObjectStore identity invalid")
	v.Must(cnpgBackupDNSName(m.ServerName), "CNPG backup server identity invalid")
	v.Must(cnpgBackupWAL.MatchString(m.EndWAL) && !strings.HasPrefix(m.EndWAL, "00000000"), "CNPG backup end WAL invalid")
	v.Must(cnpgBackupLSN.MatchString(m.EndLSN), "CNPG backup end LSN invalid")
	return WrapValidate(v)
}

// Only observations of an exact native Backup have a result. Disconnects, missing
// resources and ambiguous errors retain the active operation; they are not failed.
type DeployerSetResultCNPGBackup struct {
	DeployerCommandCNPGBackup
	Status      CNPGBackupStatus    `json:"status"`
	Material    *CNPGBackupMaterial `json:"material,omitempty"`
	FailureCode CNPGFailureCode     `json:"failureCode,omitempty"`
}

func (m DeployerSetResultCNPGBackup) Valid() error {
	if err := m.DeployerCommandCNPGBackup.Valid(); err != nil {
		return err
	}
	v := validator.New()
	v.Must(cnpgBackupUUID(m.SourceClusterUID) && cnpgBackupUUID(m.BackupUID), "CNPG backup native identity required")
	switch m.Status {
	case CNPGBackupRunning:
		v.Must(m.Material == nil && m.FailureCode == "", "running CNPG backup cannot advertise terminal evidence")
	case CNPGBackupCompleted:
		v.Must(m.Material != nil && m.FailureCode == "", "completed CNPG backup recovery material required")
		if m.Material != nil {
			if err := m.Material.Valid(); err != nil {
				return err
			}
		}
	case CNPGBackupFailed:
		v.Must(m.Material == nil && m.FailureCode == CNPGFailureBackup, "CNPG backup failure invalid")
	default:
		v.Must(false, "CNPG backup status invalid")
	}
	return WrapValidate(v)
}

func cnpgBackupUUID(value string) bool {
	return len(value) == 36 && value != "00000000-0000-0000-0000-000000000000" && govalidator.IsUUID(value)
}

func cnpgBackupDNSName(value string) bool {
	if value == "" || len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) > 63 || !cnpgBackupDNSLabel.MatchString(label) {
			return false
		}
	}
	return true
}
