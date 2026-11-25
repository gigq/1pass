// Repositories that read a 1Password 7 SQLite vault when the vault path points
// at one, and fall back to the OPVault file repositories otherwise.
//
// @author TSS

package sqlite

import (
	"log"
	"sync"

	"github.com/mashmb/1pass/1pass-core/core/domain"
	"github.com/mashmb/1pass/1pass-core/port/out"
)

type cache struct {
	mu      sync.Mutex
	dbPath  string
	profile *Profile
	items   []*domain.RawItem
}

func (c *cache) load(dbPath string) (*Profile, []*domain.RawItem) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.dbPath != dbPath || c.profile == nil {
		profile, items, err := Load(dbPath)

		if err != nil {
			log.Fatalln(err)
		}

		c.dbPath, c.profile, c.items = dbPath, profile, items
	}

	return c.profile, c.items
}

var shared = &cache{}

type autoProfileRepo struct {
	file    out.ProfileRepo
	profile *Profile
}

// NewAutoProfileRepo wraps the OPVault profile repository.
func NewAutoProfileRepo(file out.ProfileRepo) *autoProfileRepo {
	return &autoProfileRepo{file: file}
}

func (repo *autoProfileRepo) GetIterations() int {
	if repo.profile != nil {
		return repo.profile.Iterations
	}

	return repo.file.GetIterations()
}

func (repo *autoProfileRepo) GetMasterKey() string {
	if repo.profile != nil {
		return repo.profile.MasterKey
	}

	return repo.file.GetMasterKey()
}

func (repo *autoProfileRepo) GetOverviewKey() string {
	if repo.profile != nil {
		return repo.profile.OverviewKey
	}

	return repo.file.GetOverviewKey()
}

func (repo *autoProfileRepo) GetSalt() string {
	if repo.profile != nil {
		return repo.profile.Salt
	}

	return repo.file.GetSalt()
}

func (repo *autoProfileRepo) LoadProfile(vault *domain.Vault) {
	if dbPath, ok := domain.SqliteVaultPath(vault.Path); ok {
		repo.profile, _ = shared.load(dbPath)
		return
	}

	repo.profile = nil
	repo.file.LoadProfile(vault)
}

type autoItemRepo struct {
	out.ItemRepo
}

// NewAutoItemRepo wraps the OPVault item repository; searching and storing
// decoded items stay with it, only loading and saving are routed here.
func NewAutoItemRepo(file out.ItemRepo) *autoItemRepo {
	return &autoItemRepo{ItemRepo: file}
}

func (repo *autoItemRepo) LoadItems(vault *domain.Vault) []*domain.RawItem {
	if dbPath, ok := domain.SqliteVaultPath(vault.Path); ok {
		_, items := shared.load(dbPath)
		return items
	}

	return repo.ItemRepo.LoadItems(vault)
}

func (repo *autoItemRepo) SaveItem(vault *domain.Vault, uid string, item map[string]interface{}) error {
	if vault != nil {
		if _, ok := domain.SqliteVaultPath(vault.Path); ok {
			return ErrReadOnly
		}
	}

	return repo.ItemRepo.SaveItem(vault, uid, item)
}
