// Default implementation of item service.
//
// @author TSS

package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mashmb/1pass/1pass-core/core/domain"
	"github.com/mashmb/1pass/1pass-core/port/out"
)

type dfltItemService struct {
	keyService KeyService
	itemRepo   out.ItemRepo
}

func NewDfltItemService(keyService KeyService, itemRepo out.ItemRepo) *dfltItemService {
	return &dfltItemService{
		keyService: keyService,
		itemRepo:   itemRepo,
	}
}

func (s *dfltItemService) ClearMemory() {
	s.itemRepo.RemoveItems()
}

func (s *dfltItemService) CountItems(category *domain.ItemCategory, trashed bool) int {
	return s.itemRepo.CountByCategoryAndTrashed(category, trashed)
}

func (s *dfltItemService) DecodeDetails(encoded *domain.RawItem, keys *domain.Keys) map[string]interface{} {
	var detailsJson map[string]interface{}
	detailsData, _ := base64.StdEncoding.DecodeString(encoded.Details)
	itemKey, itemMac := s.keyService.ItemKeys(encoded, keys)
	details, _ := s.keyService.DecodeOpdata(detailsData, itemKey, itemMac)
	json.Unmarshal(details, &detailsJson)

	return detailsJson
}

func (s *dfltItemService) DecodeItems(vault *domain.Vault, keys *domain.Keys) {
	items := make([]*domain.Item, 0)
	encodedItems := s.itemRepo.LoadItems(vault)

	for _, encoded := range encodedItems {
		cat, err := domain.ItemCategoryEnum.FromCode(encoded.Category)

		if err == nil {
			overviewJson := s.DecodeOverview(encoded, keys)
			detailsJson := s.DecodeDetails(encoded, keys)
			sections := make([]*domain.ItemSection, 0)

			if detailsJson["sections"] == nil {
				if detailsJson["fields"] != nil {
					fieldsJson := detailsJson["fields"].([]interface{})
					fields := make([]*domain.ItemField, 0)

					for _, fieldJson := range fieldsJson {
						field := s.ParseItemField(false, fieldJson.(map[string]interface{}))

						if field != nil {
							fields = append(fields, field)
						}
					}

					if len(fields) != 0 {
						section := domain.NewItemSection("", fields)
						sections = append(sections, section)
					}
				}
			} else {
				sectionsJson := detailsJson["sections"].([]interface{})

				for _, sectionJson := range sectionsJson {
					section := s.ParseItemSection(sectionJson.(map[string]interface{}))

					if section != nil {
						sections = append(sections, section)
					}
				}
			}

			var title string
			var url string
			var notes string

			if overviewJson["title"] != nil {
				title = overviewJson["title"].(string)
			}

			if overviewJson["url"] != nil {
				url = overviewJson["url"].(string)
			}

			if detailsJson["notesPlain"] != nil {
				notes = detailsJson["notesPlain"].(string)
			}

			if len(sections) == 0 {
				sections = nil
			}

			item := domain.NewItem(encoded.Uid, title, url, notes, encoded.Trashed, cat, sections, encoded.Created, encoded.Updated)
			item.Overview = overviewJson
			item.Details = detailsJson
			item.Raw = encoded.Raw
			items = append(items, item)
		}
	}

	s.itemRepo.StoreItems(items)
}

func (s *dfltItemService) DecodeOverview(encoded *domain.RawItem, keys *domain.Keys) map[string]interface{} {
	var overviewJson map[string]interface{}
	overviewData, _ := base64.StdEncoding.DecodeString(encoded.Overview)
	overview, _ := s.keyService.DecodeOpdata(overviewData, keys.OverviewKey, keys.OverviewMac)
	json.Unmarshal(overview, &overviewJson)

	return overviewJson
}

func (s *dfltItemService) GetItem(uid string, trashed bool) *domain.Item {
	return s.itemRepo.FindFirstByUidAndTrashed(uid, trashed)
}

func (s *dfltItemService) GetSimpleItems(category *domain.ItemCategory, title string, trashed bool) []*domain.SimpleItem {
	items := make([]*domain.SimpleItem, 0)
	decodedItems := s.itemRepo.FindByCategoryAndTitleAndTrashed(category, title, trashed)

	for _, decoded := range decodedItems {
		item := domain.NewSimpleItem(decoded.Category, decoded.Title, decoded.Trashed, decoded.Uid)
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Category.GetCode() != items[j].Category.GetCode() {
			return items[i].Category.GetCode() < items[j].Category.GetCode()
		}

		return items[i].Title < items[j].Title
	})

	return items
}

func (s *dfltItemService) ParseItemField(fromSection bool, data map[string]interface{}) *domain.ItemField {
	var field *domain.ItemField
	var value string

	if !fromSection {
		if data["value"] != nil && data["name"] != nil {
			value = data["value"].(string)
			name := data["name"].(string)

			if value != "" && name != "" {
				field = domain.NewItemField(strings.Title(name), value)
			}
		}
	} else {
		if data["v"] != nil {
			dataType, err := domain.DataTypeEnum.FromName(data["k"].(string))

			if err != nil {
				value = data["v"].(string)
			} else {
				if dataType == domain.DataTypeEnum.Address {
					value = domain.DataTypeEnum.ParseValue(dataType, "", data["v"].(map[string]interface{}))
				} else {
					value = domain.DataTypeEnum.ParseValue(dataType, fmt.Sprint(data["v"]), nil)
				}
			}

			field = domain.NewItemField(strings.Title(data["t"].(string)), value)
		}
	}

	return field
}

func (s *dfltItemService) ParseItemSection(data map[string]interface{}) *domain.ItemSection {
	var title string
	fields := make([]*domain.ItemField, 0)

	if data["fields"] != nil {
		fieldsData := data["fields"].([]interface{})

		for _, fieldData := range fieldsData {
			field := s.ParseItemField(true, fieldData.(map[string]interface{}))

			if field != nil {
				fields = append(fields, field)
			}
		}
	}

	if len(fields) == 0 {
		fields = nil
	}

	if data["title"] != nil {
		title = strings.Title(data["title"].(string))
	}

	if fields == nil && title == "" {
		return nil
	}

	return domain.NewItemSection(strings.Title(title), fields)
}

func (s *dfltItemService) UpdateItem(vault *domain.Vault, keys *domain.Keys, item *domain.Item, payload *domain.ItemPayload) error {
	if vault == nil || keys == nil || item == nil || payload == nil || payload.Overview == nil || payload.Details == nil {
		return domain.ErrInvalidPayload
	}

	raw := cloneMap(item.Raw)

	if raw == nil {
		raw = make(map[string]interface{})
	}

	mergeMeta(raw, payload.Meta)

	category, err := s.normalizeCategory(payload.Meta, item.Category)

	if err != nil {
		return err
	}

	trashed := normalizeBool(payload.Meta, "trashed", item.Trashed)
	created := item.Created

	if created == 0 {
		created = normalizeInt(raw["created"], 0)
	}

	if created == 0 {
		created = time.Now().Unix()
	}

	updated := time.Now().Unix()
	rawItem, err := rawItemFromMap(item.Uid, raw)

	if err != nil {
		return err
	}

	itemKey, itemMac := s.keyService.ItemKeys(rawItem, keys)
	encodedOverview, err := s.encodeOverview(payload.Overview, keys)

	if err != nil {
		return err
	}

	encodedDetails, err := s.encodeDetails(payload.Details, itemKey, itemMac)

	if err != nil {
		return err
	}

	raw["uuid"] = item.Uid
	raw["category"] = category.GetCode()
	raw["created"] = created
	raw["updated"] = updated
	raw["tx"] = updated
	raw["o"] = encodedOverview
	raw["d"] = encodedDetails
	raw["k"] = rawItem.Keys

	if trashed {
		raw["trashed"] = true
	} else {
		if _, exists := raw["trashed"]; exists {
			raw["trashed"] = false
		} else {
			delete(raw, "trashed")
		}
	}

	itemHmac, err := s.computeItemHmac(raw, keys.OverviewMac)

	if err != nil {
		return err
	}

	raw["hmac"] = itemHmac

	return s.itemRepo.SaveItem(vault, item.Uid, raw)
}

func (s *dfltItemService) CreateItem(vault *domain.Vault, keys *domain.Keys, payload *domain.ItemPayload) (string, error) {
	if vault == nil || keys == nil || payload == nil || payload.Overview == nil || payload.Details == nil {
		return "", domain.ErrInvalidPayload
	}

	uid, err := newUuid()

	if err != nil {
		return "", err
	}

	raw := make(map[string]interface{})
	mergeMeta(raw, payload.Meta)

	category, err := s.normalizeCategory(payload.Meta, domain.ItemCategoryEnum.SecureNote)

	if err != nil {
		return "", err
	}

	trashed := normalizeBool(payload.Meta, "trashed", false)
	created := time.Now().Unix()
	updated := created
	itemKey, err := randomBytes(32)

	if err != nil {
		return "", err
	}

	itemMac, err := randomBytes(32)

	if err != nil {
		return "", err
	}

	encryptedKeys, err := s.keyService.EncryptItemKeys(itemKey, itemMac, keys)

	if err != nil {
		return "", err
	}

	encodedOverview, err := s.encodeOverview(payload.Overview, keys)

	if err != nil {
		return "", err
	}

	encodedDetails, err := s.encodeDetails(payload.Details, itemKey, itemMac)

	if err != nil {
		return "", err
	}

	raw["uuid"] = uid
	raw["category"] = category.GetCode()
	raw["created"] = created
	raw["updated"] = updated
	raw["tx"] = updated
	raw["k"] = base64.StdEncoding.EncodeToString(encryptedKeys)
	raw["o"] = encodedOverview
	raw["d"] = encodedDetails

	if trashed {
		raw["trashed"] = true
	}

	itemHmac, err := s.computeItemHmac(raw, keys.OverviewMac)

	if err != nil {
		return "", err
	}

	raw["hmac"] = itemHmac

	if err := s.itemRepo.SaveItem(vault, uid, raw); err != nil {
		return "", err
	}

	return uid, nil
}

func (s *dfltItemService) encodeOverview(overview map[string]interface{}, keys *domain.Keys) (string, error) {
	data, err := json.Marshal(overview)

	if err != nil {
		return "", err
	}

	encoded, err := s.keyService.EncodeOpdata(data, keys.OverviewKey, keys.OverviewMac)

	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(encoded), nil
}

func (s *dfltItemService) encodeDetails(details map[string]interface{}, itemKey, itemMac []byte) (string, error) {
	data, err := json.Marshal(details)

	if err != nil {
		return "", err
	}

	encoded, err := s.keyService.EncodeOpdata(data, itemKey, itemMac)

	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(encoded), nil
}

func (s *dfltItemService) computeItemHmac(raw map[string]interface{}, macKey []byte) (string, error) {
	if raw == nil {
		return "", domain.ErrInvalidPayload
	}

	keys := make([]string, 0, len(raw))

	for key := range raw {
		if key == "hmac" {
			continue
		}

		keys = append(keys, key)
	}

	sort.Strings(keys)
	hash := hmac.New(sha256.New, macKey)

	for _, key := range keys {
		if _, err := hash.Write([]byte(key)); err != nil {
			return "", err
		}

		val := valueString(raw[key])

		if _, err := hash.Write([]byte(val)); err != nil {
			return "", err
		}
	}

	return base64.StdEncoding.EncodeToString(hash.Sum(nil)), nil
}

func (s *dfltItemService) normalizeCategory(meta map[string]interface{}, fallback *domain.ItemCategory) (*domain.ItemCategory, error) {
	if meta != nil {
		if raw, ok := meta["category"]; ok {
			if value, ok := raw.(string); ok {
				value = strings.TrimSpace(value)

				if value != "" {
					if isNumeric(value) && len(value) == 3 {
						return domain.ItemCategoryEnum.FromCode(value)
					}

					return domain.ItemCategoryEnum.FromName(value)
				}
			}
		}
	}

	if fallback != nil {
		return fallback, nil
	}

	return domain.ItemCategoryEnum.SecureNote, nil
}

func normalizeBool(meta map[string]interface{}, key string, fallback bool) bool {
	if meta == nil {
		return fallback
	}

	value, ok := meta[key]

	if !ok {
		return fallback
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		v = strings.TrimSpace(strings.ToLower(v))

		if v == "true" || v == "1" || v == "yes" || v == "y" {
			return true
		}

		if v == "false" || v == "0" || v == "no" || v == "n" {
			return false
		}
	case float64:
		return v != 0
	case int:
		return v != 0
	case int64:
		return v != 0
	}

	return fallback
}

func normalizeInt(value interface{}, fallback int64) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		if v == math.Trunc(v) {
			return int64(v)
		}
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return parsed
		}
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)

		if err == nil {
			return parsed
		}
	}

	return fallback
}

func rawItemFromMap(uid string, raw map[string]interface{}) (*domain.RawItem, error) {
	if raw == nil {
		return nil, domain.ErrInvalidPayload
	}

	keys, ok := raw["k"].(string)

	if !ok || keys == "" {
		return nil, domain.ErrInvalidPayload
	}

	category, _ := raw["category"].(string)
	details, _ := raw["d"].(string)
	hmacVal, _ := raw["hmac"].(string)
	overview, _ := raw["o"].(string)
	created := normalizeInt(raw["created"], 0)
	updated := normalizeInt(raw["updated"], 0)
	trashed := normalizeBool(raw, "trashed", false)
	item := domain.NewRawItem(category, details, hmacVal, keys, overview, uid, created, updated, trashed)
	item.Raw = raw

	return item, nil
}

func mergeMeta(dest map[string]interface{}, meta map[string]interface{}) {
	if dest == nil || meta == nil {
		return
	}

	for key, value := range meta {
		if isReservedMetaKey(key) {
			continue
		}

		dest[key] = value
	}
}

func isReservedMetaKey(key string) bool {
	switch key {
	case "d", "o", "k", "hmac", "overview", "details":
		return true
	default:
		return false
	}
}

func cloneMap(source map[string]interface{}) map[string]interface{} {
	if source == nil {
		return nil
	}

	clone := make(map[string]interface{}, len(source))

	for key, value := range source {
		clone[key] = value
	}

	return clone
}

func valueString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "1"
		}
		return "0"
	case int:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		if v == math.Trunc(v) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	case json.Number:
		return v.String()
	case nil:
		return ""
	default:
		encoded, err := json.Marshal(v)

		if err != nil {
			return fmt.Sprint(v)
		}

		return string(encoded)
	}
}

func randomBytes(size int) ([]byte, error) {
	if size <= 0 {
		return nil, domain.ErrInvalidPayload
	}

	data := make([]byte, size)

	if _, err := rand.Read(data); err != nil {
		return nil, err
	}

	return data, nil
}

func newUuid() (string, error) {
	data, err := randomBytes(16)

	if err != nil {
		return "", err
	}

	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80

	return strings.ToUpper(hex.EncodeToString(data)), nil
}

func isNumeric(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}
