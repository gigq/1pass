// Read-only access to 1Password 7 SQLite (CloudKit / iCloud) vaults.
//
// 1Password 7 keeps its primary vault in OnePassword.sqlite. The profile and
// item blobs use the same opdata01 / key layout as OPVault, so they can be fed
// to the existing key and item services after base64 encoding.
//
// The database is never opened in place: it (and its -wal / -shm files) is
// copied to a private temporary directory first, so a running 1Password app
// is never disturbed and the vault stays read-only.
//
// @author TSS

package sqlite

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mashmb/1pass/1pass-core/core/domain"

	_ "modernc.org/sqlite"
)

// ProfileEnv selects a profile (vault) by name or UUID; "default" otherwise.
const ProfileEnv string = "ONEPASS_SQLITE_PROFILE"

// ErrReadOnly is returned when writing to a SQLite vault is attempted.
var ErrReadOnly = errors.New("1Password 7 SQLite vaults are read-only; add or edit items in 1Password")

type Profile struct {
	Iterations  int
	MasterKey   string
	OverviewKey string
	Salt        string
}

// Load copies the database, then reads the selected profile and its items.
func Load(dbPath string) (*Profile, []*domain.RawItem, error) {
	tmpDir, err := os.MkdirTemp("", "1pass-sqlite-")

	if err != nil {
		return nil, nil, err
	}

	defer os.RemoveAll(tmpDir)

	copyPath := filepath.Join(tmpDir, filepath.Base(dbPath))

	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := copyFile(dbPath+suffix, copyPath+suffix); err != nil {
			if suffix != "" && errors.Is(err, os.ErrNotExist) {
				continue
			}

			return nil, nil, err
		}
	}

	db, err := sql.Open("sqlite", copyPath)

	if err != nil {
		return nil, nil, err
	}

	defer db.Close()

	profileId, profile, err := loadProfile(db)

	if err != nil {
		return nil, nil, err
	}

	items, err := loadItems(db, profileId)

	if err != nil {
		return nil, nil, err
	}

	return profile, items, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)

	if err != nil {
		return err
	}

	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)

	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}

func loadProfile(db *sql.DB) (int64, *Profile, error) {
	want := strings.TrimSpace(os.Getenv(ProfileEnv))

	if want == "" {
		want = "default"
	}

	rows, err := db.Query(`SELECT id, uuid, profile_name, iterations, salt, master_key_data, overview_key_data
		FROM profiles WHERE trashed = 0 ORDER BY id`)

	if err != nil {
		return 0, nil, err
	}

	defer rows.Close()

	var firstId int64 = -1
	var first *Profile

	for rows.Next() {
		var id int64
		var uuid, name string
		var iterations int
		var salt, masterKey, overviewKey []byte

		if err := rows.Scan(&id, &uuid, &name, &iterations, &salt, &masterKey, &overviewKey); err != nil {
			return 0, nil, err
		}

		profile := &Profile{
			Iterations:  iterations,
			MasterKey:   base64.StdEncoding.EncodeToString(masterKey),
			OverviewKey: base64.StdEncoding.EncodeToString(overviewKey),
			Salt:        base64.StdEncoding.EncodeToString(salt),
		}

		if strings.EqualFold(name, want) || strings.EqualFold(uuid, want) {
			return id, profile, nil
		}

		if first == nil {
			firstId, first = id, profile
		}
	}

	if err := rows.Err(); err != nil {
		return 0, nil, err
	}

	if first != nil && os.Getenv(ProfileEnv) == "" {
		return firstId, first, nil
	}

	return 0, nil, fmt.Errorf("no 1Password profile named %q in the SQLite vault", want)
}

func loadItems(db *sql.DB, profileId int64) ([]*domain.RawItem, error) {
	rows, err := db.Query(`SELECT i.uuid, i.category_uuid, i.created_at, i.updated_at, i.trashed,
			i.key_data, i.overview_data, d.data
		FROM items i LEFT JOIN item_details d ON d.item_id = i.id
		WHERE i.profile_id = ? ORDER BY i.id`, profileId)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	items := make([]*domain.RawItem, 0)

	for rows.Next() {
		var uid, category string
		var created, updated int64
		var trashed int
		var keys, overview, details []byte

		if err := rows.Scan(&uid, &category, &created, &updated, &trashed, &keys, &overview, &details); err != nil {
			return nil, err
		}

		// Tombstones and items without keys or details cannot be decoded.
		if len(keys) == 0 || len(overview) == 0 || len(details) == 0 {
			continue
		}

		enc := base64.StdEncoding.EncodeToString
		item := domain.NewRawItem(category, enc(details), "", enc(keys), enc(overview), uid, created, updated,
			trashed != 0)
		items = append(items, item)
	}

	return items, rows.Err()
}
