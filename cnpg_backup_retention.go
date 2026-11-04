package api

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/moonrhythm/validator"
)

// CNPGBackupArchive binds new material to a location-owned archive configuration.
// It contains no credentials and never authorizes access or historical adoption.
type CNPGBackupArchive struct {
	EndpointURL       string `json:"endpointURL"`
	DestinationPath   string `json:"destinationPath"`
	ProfileRevision   int64  `json:"profileRevision"`
	ConfigurationHash string `json:"configurationHash"`
}

func (m CNPGBackupArchive) Valid() error {
	v := validator.New()
	endpoint, err := url.Parse(m.EndpointURL)
	v.Must(err == nil && len(m.EndpointURL) <= 2048 && plainArchiveURL(endpoint) &&
		(endpoint.Scheme == "http" || endpoint.Scheme == "https") &&
		(endpoint.Path == "" || endpoint.Path == "/"), "CNPG archive endpoint invalid")
	destination, err := url.Parse(m.DestinationPath)
	validDestination := err == nil && len(m.DestinationPath) <= 2048 && plainArchiveURL(destination) &&
		destination.Scheme == "s3" && destination.Port() == "" && cnpgBackupDNSName(destination.Host)
	if validDestination {
		for _, part := range strings.Split(strings.Trim(destination.Path, "/"), "/") {
			if part == "." || part == ".." {
				validDestination = false
			}
		}
		validDestination = validDestination && !strings.Contains(destination.Path, "//") && !strings.ContainsAny(destination.Path, "\\%")
	}
	v.Must(validDestination, "CNPG archive destination invalid")
	v.Must(m.ProfileRevision > 0 && cnpgReadinessConfigurationHash.MatchString(m.ConfigurationHash), "CNPG archive profile binding invalid")
	return WrapValidate(v)
}

func plainArchiveURL(u *url.URL) bool {
	return u != nil && u.Hostname() != "" && u.User == nil && u.Opaque == "" &&
		u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && u.RawPath == ""
}

type CNPGBackupRetentionItem struct {
	OperationID string             `json:"operationId"`
	BackupUID   string             `json:"backupUid"`
	Material    CNPGBackupMaterial `json:"material"`
}

// OperationID identifies retention, not a Backup. Exactly eight successes are
// bound in native backup-ID order; the keep policy is fixed at seven.
type DeployerCommandCNPGBackupRetention struct {
	DeployerCommandCNPGIdentity
	CreateOperationID      string                    `json:"createOperationId"`
	SourceClusterUID       string                    `json:"sourceClusterUid"`
	ReplacementOperationID string                    `json:"replacementOperationId"`
	Backups                []CNPGBackupRetentionItem `json:"backups"`
	JobUID                 string                    `json:"jobUid,omitempty"`
}

func (m DeployerCommandCNPGBackupRetention) Valid() error {
	if err := m.DeployerCommandCNPGIdentity.Valid(); err != nil {
		return err
	}
	v := validator.New()
	v.Must(cnpgBackupUUID(m.CreateOperationID) && !strings.EqualFold(m.CreateOperationID, m.OperationID), "CNPG retention source operation invalid")
	v.Must(cnpgBackupUUID(m.SourceClusterUID), "CNPG retention source checkpoint required")
	v.Must(cnpgBackupUUID(m.ReplacementOperationID), "CNPG retention replacement invalid")
	v.Must(m.JobUID == "" || cnpgBackupUUID(m.JobUID), "CNPG retention Job checkpoint invalid")
	v.Must(len(m.Backups) == 8, "CNPG retention requires eight backups")
	if err := WrapValidate(v); err != nil {
		return err
	}
	operations := map[string]bool{strings.ToLower(m.OperationID): true, strings.ToLower(m.CreateOperationID): true}
	uids := make(map[string]bool, 8)
	first := m.Backups[0].Material
	for i, b := range m.Backups {
		if err := b.Material.Valid(); err != nil {
			return err
		}
		op, uid := strings.ToLower(b.OperationID), strings.ToLower(b.BackupUID)
		v.Must(cnpgBackupUUID(b.OperationID) && !operations[op], "CNPG retention backup operation invalid or repeated")
		v.Must(cnpgBackupUUID(b.BackupUID) && !uids[uid], "CNPG retention native backup invalid or repeated")
		operations[op], uids[uid] = true, true
		v.Must(b.Material.Archive != nil, "CNPG retention archive binding required")
		if first.Archive != nil && b.Material.Archive != nil {
			v.Must(*b.Material.Archive == *first.Archive, "CNPG retention archive binding mismatch")
		}
		v.Must(b.Material.ObjectStoreName == first.ObjectStoreName && b.Material.ObjectStoreUID == first.ObjectStoreUID &&
			b.Material.ServerName == fmt.Sprintf("cnpg-%d-%d", m.ID, m.ProjectID) &&
			b.Material.Image == first.Image && b.Material.StorageClass == first.StorageClass && b.Material.MajorVersion == first.MajorVersion,
			"CNPG retention backup profile mismatch")
		if i > 0 {
			v.Must(m.Backups[i-1].Material.BarmanBackupID < b.Material.BarmanBackupID, "CNPG retention backup order invalid")
		}
	}
	v.Must(strings.EqualFold(m.Backups[7].OperationID, m.ReplacementOperationID), "CNPG retention replacement must be newest")
	return WrapValidate(v)
}

type CNPGBackupRetentionStatus string

const (
	CNPGBackupRetentionRunning   CNPGBackupRetentionStatus = "running"
	CNPGBackupRetentionBlocked   CNPGBackupRetentionStatus = "blocked"
	CNPGBackupRetentionCompleted CNPGBackupRetentionStatus = "completed"
	CNPGFailureRetentionScope    CNPGFailureCode           = "retention_scope"
	CNPGFailureRetentionJob      CNPGFailureCode           = "retention_job"
	CNPGFailureRetentionObserve  CNPGFailureCode           = "retention_observation"
)

// Completion describes stock lifecycle/catalog observation, never WAL erasure.
// Blocked keeps the operation guard; transport uncertainty is not completion.
type DeployerSetResultCNPGBackupRetention struct {
	DeployerCommandCNPGBackupRetention
	Status      CNPGBackupRetentionStatus `json:"status"`
	PodUIDs     []string                  `json:"podUids,omitempty"`
	RetainedIDs []string                  `json:"retainedIds,omitempty"`
	ExpiredIDs  []string                  `json:"expiredIds,omitempty"`
	FailureCode CNPGFailureCode           `json:"failureCode,omitempty"`
}

func (m DeployerSetResultCNPGBackupRetention) Valid() error {
	if err := m.DeployerCommandCNPGBackupRetention.Valid(); err != nil {
		return err
	}
	v := validator.New()
	v.Must(len(m.PodUIDs) <= 8, "CNPG retention Pod evidence too large")
	if err := WrapValidate(v); err != nil {
		return err
	}
	uids := make(map[string]bool, len(m.PodUIDs))
	for _, uid := range m.PodUIDs {
		key := strings.ToLower(uid)
		v.Must(cnpgBackupUUID(uid) && !uids[key], "CNPG retention Pod identity invalid or repeated")
		uids[key] = true
	}
	v.Must(len(m.PodUIDs) == 0 || m.JobUID != "", "CNPG retention Pods require Job checkpoint")
	switch m.Status {
	case CNPGBackupRetentionRunning:
		v.Must(m.JobUID != "" && m.FailureCode == "" && len(m.RetainedIDs) == 0 && len(m.ExpiredIDs) == 0, "CNPG running retention evidence invalid")
	case CNPGBackupRetentionBlocked:
		v.Must((m.FailureCode == CNPGFailureRetentionScope || m.FailureCode == CNPGFailureRetentionJob || m.FailureCode == CNPGFailureRetentionObserve) &&
			len(m.RetainedIDs) == 0 && len(m.ExpiredIDs) == 0, "CNPG blocked retention evidence invalid")
	case CNPGBackupRetentionCompleted:
		v.Must(m.JobUID != "" && len(m.PodUIDs) == 1 && m.FailureCode == "" && len(m.RetainedIDs) == 7 && len(m.ExpiredIDs) == 1, "CNPG completed retention evidence invalid")
		if len(m.RetainedIDs) == 7 && len(m.ExpiredIDs) == 1 {
			v.Must(m.ExpiredIDs[0] == m.Backups[0].Material.BarmanBackupID, "CNPG retention expired identity mismatch")
			for i, id := range m.RetainedIDs {
				v.Must(id == m.Backups[i+1].Material.BarmanBackupID, "CNPG retention retained identity mismatch")
			}
		}
	default:
		v.Must(false, "CNPG retention status invalid")
	}
	return WrapValidate(v)
}
