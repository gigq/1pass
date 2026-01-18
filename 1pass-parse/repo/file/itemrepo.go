// Implementation of file item repository.
//
// @author TSS

package file

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"github.com/mashmb/1pass/1pass-core/core/domain"
)

type fileItemRepo struct {
	items []*domain.Item
}

func NewFileItemRepo() *fileItemRepo {
	return &fileItemRepo{}
}

func (repo *fileItemRepo) CountByCategoryAndTrashed(category *domain.ItemCategory, trashed bool) int {
	count := 0

	for _, item := range repo.items {
		if category == nil {
			if item.Trashed == trashed {
				count++
			}
		} else {
			if item.Category == category && item.Trashed == trashed {
				count++
			}
		}
	}

	return count
}

func (repo *fileItemRepo) FindByCategoryAndTitleAndTrashed(category *domain.ItemCategory, title string, trashed bool) []*domain.Item {
	title = strings.ToLower(title)
	resultSet := make([]*domain.Item, 0)

	for _, item := range repo.items {
		if category == nil {
			if title == "" {
				if item.Trashed == trashed {
					resultSet = append(resultSet, item)
				}
			} else {
				if strings.Contains(strings.ToLower(item.Title), title) && item.Trashed == trashed {
					resultSet = append(resultSet, item)
				}
			}
		} else {
			if title == "" {
				if item.Category == category && item.Trashed == trashed {
					resultSet = append(resultSet, item)
				}
			} else {
				if strings.Contains(strings.ToLower(item.Title), title) && item.Category == category && item.Trashed == trashed {
					resultSet = append(resultSet, item)
				}
			}
		}
	}

	return resultSet
}

func (repo *fileItemRepo) FindFirstByUidAndTrashed(uid string, trashed bool) *domain.Item {
	var result *domain.Item

	for _, item := range repo.items {
		if item.Uid == uid && item.Trashed == trashed {
			result = item
			break
		}
	}

	return result
}

func (repo *fileItemRepo) LoadItems(vault *domain.Vault) []*domain.RawItem {
	items := make([]*domain.RawItem, 0)
	var itemsJson map[string]interface{}
	bandFiles, err := filepath.Glob(filepath.Join(vault.Path, domain.ProfileDir, domain.BandFilePattern))

	if err != nil {
		log.Fatalln(err)
	}

	if len(bandFiles) != 0 {
		itemsFile := "{"

		for _, bandFile := range bandFiles {
			file, err := ioutil.ReadFile(bandFile)

			if err != nil {
				log.Fatalln(err)
			}

			itemsFile = itemsFile + string(file[4:len(file)-3]) + ","
		}

		itemsFile = itemsFile[:len(itemsFile)-1] + "}"

		if err := json.Unmarshal([]byte(itemsFile), &itemsJson); err != nil {
			log.Fatalln(err)
		}

		for uid, value := range itemsJson {
			v := value.(map[string]interface{})
			if v["uuid"] == nil {
				v["uuid"] = uid
			}
			created := int64(v["created"].(float64))
			updated := int64(v["updated"].(float64))
			trashed := false

			if v["trashed"] != nil {
				trashed = v["trashed"].(bool)
			}

			item := domain.NewRawItem(v["category"].(string), v["d"].(string), v["hmac"].(string), v["k"].(string),
				v["o"].(string), uid, created, updated, trashed)
			item.Raw = v
			items = append(items, item)
		}
	}

	return items
}

func (repo *fileItemRepo) RemoveItems() {
	if repo.items != nil {
		repo.items = nil
	}
}

func (repo *fileItemRepo) StoreItems(items []*domain.Item) {
	repo.items = items
}

func (repo *fileItemRepo) SaveItem(vault *domain.Vault, uid string, item map[string]interface{}) error {
	if vault == nil {
		return domain.ErrInvalidVault
	}

	if uid == "" {
		return domain.ErrInvalidPayload
	}

	band := strings.ToUpper(uid[:1])
	bandFile := filepath.Join(vault.Path, domain.ProfileDir, "band_"+band+".js")
	itemsJson := make(map[string]interface{})

	if file, err := ioutil.ReadFile(bandFile); err == nil {
		content := strings.TrimSpace(string(file))
		content = strings.TrimPrefix(content, "ld(")
		content = strings.TrimSuffix(content, ");")

		if content != "" {
			if err := json.Unmarshal([]byte(content), &itemsJson); err != nil {
				return err
			}
		}
	}

	itemsJson[uid] = item

	payload, err := json.Marshal(itemsJson)
	if err != nil {
		return err
	}

	content := "ld(" + string(payload) + ");"
	return ioutil.WriteFile(bandFile, []byte(content), 0644)
}
