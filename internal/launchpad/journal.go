package launchpad

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// readJournal loads a rollback journal. A digest mismatch is reported as a
// warning rather than a hard failure: the digest is self-computed, so it can
// only flag accidental corruption, and a recovery path must not refuse to
// recover over a checksum it could recompute itself.
func readJournal(path string, expectedDigest ...string) (Journal, []string, error) {
	file, err := openJournalRead(path)
	if err != nil {
		return Journal{}, nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Journal{}, nil, err
	}
	const maxJournalBytes = 8 * 1024 * 1024
	if info.Size() > maxJournalBytes {
		return Journal{}, nil, errors.New("rollback journal exceeds the size limit")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxJournalBytes+1))
	if err != nil {
		return Journal{}, nil, err
	}
	if len(data) > maxJournalBytes {
		return Journal{}, nil, errors.New("rollback journal exceeds the size limit")
	}
	if len(expectedDigest) > 0 && expectedDigest[0] != "" {
		sum := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(sum[:]), expectedDigest[0]) {
			return Journal{}, nil, errors.New("rollback journal changed after confirmation")
		}
	}
	var journal Journal
	if err := json.Unmarshal(data, &journal); err != nil {
		return Journal{}, nil, err
	}
	if journal.SchemaVersion != SchemaVersion {
		return Journal{}, nil, fmt.Errorf("unsupported journal schema %d", journal.SchemaVersion)
	}
	if strings.TrimSpace(journal.ID) == "" || len(journal.Actions) > 256 {
		return Journal{}, nil, errors.New("rollback journal has invalid identity or action count")
	}
	var warnings []string
	if journal.Digest != "" && !strings.EqualFold(journal.Digest, journalDigest(journal)) {
		warnings = append(warnings, "The rollback journal digest does not match its contents; continuing with best-effort recovery from the recorded actions.")
	}
	return journal, warnings, nil
}

func journalDigest(journal Journal) string {
	journal.Digest = ""
	data, err := json.Marshal(journal)
	if err != nil {
		panic(err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func writeJournalAtomic(path string, journal *Journal) error {
	journal.Digest = journalDigest(*journal)
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".journal-*.tmp")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
