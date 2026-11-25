// Detection of 1Password 7 SQLite (CloudKit / iCloud) vaults.
//
// @author TSS

package domain

import (
	"os"
	"path/filepath"
	"strings"
)

// SqliteVaultFile is the database 1Password 7 keeps for its primary
// (iCloud / CloudKit synced) vault, e.g.
// ~/Library/Group Containers/2BUA8C4S2C.com.agilebits/Library/Application Support/1Password/Data/OnePassword.sqlite
const SqliteVaultFile string = "OnePassword.sqlite"

// SqliteVaultPath returns the database file for vaultPath when it points at a
// 1Password 7 SQLite vault: either the .sqlite file itself or a directory that
// holds OnePassword.sqlite and is not an OPVault.
func SqliteVaultPath(vaultPath string) (string, bool) {
	info, err := os.Stat(vaultPath)

	if err != nil {
		return "", false
	}

	if !info.IsDir() {
		if strings.HasSuffix(strings.ToLower(vaultPath), ".sqlite") {
			return vaultPath, true
		}

		return "", false
	}

	if _, err := os.Stat(filepath.Join(vaultPath, ProfileDir, ProfileFile)); err == nil {
		return "", false
	}

	dbPath := filepath.Join(vaultPath, SqliteVaultFile)

	if info, err := os.Stat(dbPath); err == nil && !info.IsDir() {
		return dbPath, true
	}

	return "", false
}
