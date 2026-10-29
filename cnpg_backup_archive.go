package api

import (
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/moonrhythm/validator"
)

// CNPGBackupArchiveBinding is optional internal provenance, not storage authority.
// Absence preserves legacy Backup/Restore compatibility but cannot admit deletion.
// Format 1 pins the exact archive location and WAL timeline alongside Material.
type CNPGBackupArchiveBinding struct {
	Format          int    `json:"format"`
	EndpointURL     string `json:"endpointUrl"`
	DestinationPath string `json:"destinationPath"`
	BeginWAL        string `json:"beginWal"`
	Timeline        uint32 `json:"timeline"`
}

var cnpgArchiveBucket = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

func (b CNPGBackupArchiveBinding) Valid(m *CNPGBackupMaterial) error {
	v := validator.New()
	v.Must(b.Format == 1, "CNPG archive format invalid")
	e, eErr := url.Parse(b.EndpointURL)
	v.Must(eErr == nil && e != nil && (e.Scheme == "https" || e.Scheme == "http") && e.Hostname() != "" && e.User == nil && e.RawQuery == "" && !e.ForceQuery && e.Fragment == "" && e.Opaque == "" && e.RawPath == "" && (e.Path == "" || e.Path == "/"), "CNPG archive endpoint invalid")
	d, dErr := url.Parse(b.DestinationPath)
	v.Must(dErr == nil && d != nil && d.Scheme == "s3" && cnpgArchiveBucket.MatchString(d.Host) && !strings.Contains(d.Host, "..") && d.Port() == "" && d.User == nil && d.RawQuery == "" && !d.ForceQuery && d.Fragment == "" && d.Opaque == "" && d.RawPath == "" && (d.Path == "" || path.Clean(d.Path) == d.Path) && !strings.ContainsAny(d.Path, "*?[]\\"), "CNPG archive destination invalid")
	validWAL := m != nil && m.Valid() == nil && cnpgBackupWAL.MatchString(b.BeginWAL) && b.BeginWAL <= m.EndWAL
	if validWAL {
		timeline, err := strconv.ParseUint(b.BeginWAL[:8], 16, 32)
		validWAL = err == nil && b.Timeline != 0 && uint32(timeline) == b.Timeline && b.BeginWAL[:8] == m.EndWAL[:8]
	}
	v.Must(validWAL, "CNPG archive WAL binding invalid")
	return WrapValidate(v)
}
