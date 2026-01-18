// Default implementation of vault facade.
//
// @author TSS

package facade

import (
	"github.com/mashmb/1pass/1pass-core/core/domain"
	"github.com/mashmb/1pass/1pass-core/core/service"
)

type dfltVaultFacade struct {
	keys          *domain.Keys
	configService service.ConfigService
	itemService   service.ItemService
	keyService    service.KeyService
	vaultService  service.VaultService
}

func NewDfltVaultFacade(configService service.ConfigService, itemService service.ItemService,
	keyService service.KeyService, vaultService service.VaultService) *dfltVaultFacade {
	return &dfltVaultFacade{
		configService: configService,
		itemService:   itemService,
		keyService:    keyService,
		vaultService:  vaultService,
	}
}

func (f *dfltVaultFacade) CountItems(category *domain.ItemCategory, trashed bool) int {
	return f.itemService.CountItems(category, trashed)
}

func (f *dfltVaultFacade) GetItem(uid string, trashed bool) *domain.Item {
	return f.itemService.GetItem(uid, trashed)
}

func (f *dfltVaultFacade) GetItems(category *domain.ItemCategory, title string, trashed bool) []*domain.SimpleItem {
	return f.itemService.GetSimpleItems(category, title, trashed)
}

func (f *dfltVaultFacade) IsUnlocked() bool {
	unlocked := false

	if f.keys != nil {
		unlocked = true
	}

	return unlocked
}

func (f *dfltVaultFacade) Lock() {
	f.itemService.ClearMemory()
	f.keys = nil
}

func (f *dfltVaultFacade) UpdateItem(vault *domain.Vault, item *domain.Item, payload *domain.ItemPayload) (*domain.Item, error) {
	if f.keys == nil {
		return nil, domain.ErrVaultLocked
	}

	if err := f.itemService.UpdateItem(vault, f.keys, item, payload); err != nil {
		return nil, err
	}

	f.itemService.ClearMemory()
	f.itemService.DecodeItems(vault, f.keys)

	updated := f.itemService.GetItem(item.Uid, false)

	if updated == nil {
		updated = f.itemService.GetItem(item.Uid, true)
	}

	return updated, nil
}

func (f *dfltVaultFacade) CreateItem(vault *domain.Vault, payload *domain.ItemPayload) (*domain.Item, error) {
	if f.keys == nil {
		return nil, domain.ErrVaultLocked
	}

	uid, err := f.itemService.CreateItem(vault, f.keys, payload)

	if err != nil {
		return nil, err
	}

	f.itemService.ClearMemory()
	f.itemService.DecodeItems(vault, f.keys)

	created := f.itemService.GetItem(uid, false)

	if created == nil {
		created = f.itemService.GetItem(uid, true)
	}

	return created, nil
}

func (f *dfltVaultFacade) Unlock(vault *domain.Vault, password string) error {
	derivedKey, derivedMac, err := f.keyService.DerivedKeys(password)

	if err != nil {
		return domain.ErrInvalidPass
	}

	masterKey, masterMac, err := f.keyService.MasterKeys(derivedKey, derivedMac)

	if err != nil {
		return domain.ErrInvalidPass
	}

	overviewKey, overviewMac, err := f.keyService.OverviewKeys(derivedKey, derivedMac)

	if err != nil {
		return domain.ErrInvalidPass
	}

	f.keys = domain.NewKeys(derivedKey, derivedMac, masterKey, masterMac, overviewKey, overviewMac)
	f.itemService.DecodeItems(vault, f.keys)

	return nil
}

func (f *dfltVaultFacade) Validate(vault *domain.Vault) error {
	if err := f.vaultService.ValidateVault(vault); err != nil {
		return err
	}

	return nil
}
