// Implementation of cobra CLI control.
//
// @author TSS

package cobra

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mashmb/1pass/1pass-core/core/domain"
	"github.com/mashmb/1pass/1pass-core/core/facade"
	"golang.org/x/term"
)

type cobraCliControl struct {
	configFacade facade.ConfigFacade
	updateFacade facade.UpdateFacade
	vaultFacade  facade.VaultFacade
}

func NewCobraCliControl(configFacade facade.ConfigFacade, updateFacade facade.UpdateFacade,
	vaultFacade facade.VaultFacade) *cobraCliControl {
	return &cobraCliControl{
		configFacade: configFacade,
		updateFacade: updateFacade,
		vaultFacade:  vaultFacade,
	}
}

func (ctrl *cobraCliControl) CheckForUpdate() {
	config := ctrl.configFacade.GetConfig()

	if config.UpdateNotify {
		info, err := ctrl.updateFacade.CheckForUpdate(false)

		if err == nil {
			msg := fmt.Sprintf("New version of 1pass is available (current: %v, available: %v). Run 'sudo 1pass update' to upgrade.\n",
				domain.Version, info.Version)
			fmt.Println(msg)
		}
	}
}

func (ctrl *cobraCliControl) Configure() {
	ctrl.CheckForUpdate()
	var vaultVal string
	var notifyVal string
	var timeoutVal string
	var periodVal string
	var confirmVal string
	config := ctrl.configFacade.GetConfig()

	fmt.Println("Detailed configuration manual can be found online: https://github.com/mashmb/1pass#configuration")
	fmt.Println("Configuring 1pass:")
	fmt.Print(fmt.Sprintf("  1. Do you want to set default OPVault path? (%v) [y - for yes/n - for no]: ",
		domain.LogicValEnum.No.GetName()))
	fmt.Scanln(&confirmVal)
	confirm, err := domain.LogicValEnum.FromName(confirmVal)

	if err == nil && confirm == domain.LogicValEnum.Yes {
		fmt.Print(fmt.Sprintf("     Default OPVault path (%v): ", config.Vault))
		fmt.Scanln(&vaultVal)
		config.Vault = strings.TrimSpace(vaultVal)
	}

	fmt.Print(fmt.Sprintf("  2. Update notifications? (%v) [y - for yes/n - for no]: ",
		domain.LogicValEnum.FromValue(config.UpdateNotify).GetName()))
	fmt.Scanln(&notifyVal)
	notify, err := domain.LogicValEnum.FromName(notifyVal)

	if err == nil {
		config.UpdateNotify = notify.GetValue()
	}

	fmt.Print(fmt.Sprintf("  3. Update HTTP timeout in seconds (%d) [1-30]: ", config.Timeout))
	fmt.Scanln(&timeoutVal)
	timeout, err := strconv.ParseInt(timeoutVal, 10, 64)

	if err == nil && timeout >= 1 && timeout <= 30 {
		config.Timeout = int(timeout)
	}

	fmt.Print(fmt.Sprintf("  4. How often check for updates in days (%d) [0-365]: ", config.UpdatePeriod))
	fmt.Scanln(&periodVal)
	period, err := strconv.ParseInt(periodVal, 10, 64)

	if err == nil && period >= 0 && period <= 365 {
		config.UpdatePeriod = int(period)
	}

	ctrl.configFacade.SaveConfig(config)
	fmt.Println("1pass configured")
}

func (ctrl *cobraCliControl) FirstRun() {
	if !ctrl.configFacade.IsConfigAvailable() {
		fmt.Println("Running 1pass for the first time? Let's configure it!")
		fmt.Println()
		ctrl.Configure()
		fmt.Println()
	}
}

func (ctrl *cobraCliControl) GetCategories() {
	ctrl.CheckForUpdate()
	t := table.NewWriter()
	t.SetStyle(table.StyleDouble)
	t.AppendHeader(table.Row{"lp.", "category"})
	categories := domain.ItemCategoryEnum.GetValues()

	sort.Slice(categories, func(i, j int) bool {
		return categories[i].GetCode() < categories[j].GetCode()
	})

	for i, cat := range categories {
		t.AppendRow(table.Row{i + 1, cat.GetName()})
	}

	fmt.Println(t.Render())
}

func (ctrl *cobraCliControl) GetItemDetails(vaultPath, uid string, trashed bool) {
	ctrl.FirstRun()
	ctrl.CheckForUpdate()
	var vault *domain.Vault

	if vaultPath != "" {
		vault = domain.NewVault(vaultPath)
	} else {
		config := ctrl.configFacade.GetConfig()
		vault = domain.NewVault(config.Vault)
	}

	err := ctrl.vaultFacade.Validate(vault)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Password:")
	password, err := term.ReadPassword(int(syscall.Stdin))
	err = ctrl.vaultFacade.Unlock(vault, string(password))

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	item := ctrl.vaultFacade.GetItem(uid, trashed)

	if item != nil {
		fmt.Println()
		fmt.Println(item.Category.GetName())
		fmt.Println("------------------------------")
		fmt.Println(item.Title)
		fmt.Println()
		updated := time.Unix(item.Updated, 0).Format("2006-01-02 15:04:05")
		created := time.Unix(item.Created, 0).Format("2006-01-02 15:04:05")
		fmt.Println(fmt.Sprintf("Updated: %v\nCreated: %v\nTrashed: %v", updated, created, item.Trashed))

		if item.Url != "" {
			fmt.Println(fmt.Sprintf("URL: %v", item.Url))
		}

		if item.Sections != nil {
			for _, section := range item.Sections {
				fmt.Println()

				if section.Title != "" {
					fmt.Println(section.Title)
				}

				fmt.Println("------------------------------")

				if section.Fields != nil {
					for _, field := range section.Fields {
						fmt.Println(fmt.Sprintf("%v: %v", field.Name, field.Value))
					}
				}
			}

			fmt.Println()
		}

		if item.Notes != "" {
			if item.Sections == nil {
				fmt.Println()
			}

			fmt.Println("Notes")
			fmt.Println("------------------------------")
			fmt.Println(item.Notes)
			fmt.Println()
		}
	} else {
		msg := fmt.Sprintf("Item with UID %v do not exist", uid)
		fmt.Println(msg)
	}

}

func (ctrl *cobraCliControl) GetItemOverview(vaultPath, uid string, trashed bool) {
	ctrl.FirstRun()
	ctrl.CheckForUpdate()
	var vault *domain.Vault

	if vaultPath != "" {
		vault = domain.NewVault(vaultPath)
	} else {
		config := ctrl.configFacade.GetConfig()
		vault = domain.NewVault(config.Vault)
	}

	err := ctrl.vaultFacade.Validate(vault)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Password:")
	password, err := term.ReadPassword(int(syscall.Stdin))
	err = ctrl.vaultFacade.Unlock(vault, string(password))

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	item := ctrl.vaultFacade.GetItem(uid, trashed)

	if item != nil {
		fmt.Println()
		fmt.Println(item.Category.GetName())
		fmt.Println("------------------------------")
		fmt.Println(item.Title)
		fmt.Println()
		updated := time.Unix(item.Updated, 0).Format("2006-01-02 15:04:05")
		created := time.Unix(item.Created, 0).Format("2006-01-02 15:04:05")
		fmt.Println(fmt.Sprintf("Updated: %v\nCreated: %v\nTrashed: %v", updated, created, item.Trashed))

		if item.Url != "" {
			fmt.Println(fmt.Sprintf("URL: %v", item.Url))
		}

		if item.Sections != nil {
			for _, section := range item.Sections {
				fmt.Println()

				if section.Title != "" {
					fmt.Println(section.Title)
				}

				fmt.Println("------------------------------")

				if section.Fields != nil {
					for _, field := range section.Fields {
						fmt.Println(fmt.Sprintf("%v: %v", field.Name, "**********"))
					}
				}
			}

			fmt.Println()
		}

		if item.Notes != "" {
			if item.Sections == nil {
				fmt.Println()
			}

			fmt.Println("Notes")
			fmt.Println("------------------------------")
			fmt.Println("**********")
			fmt.Println()
		}
	} else {
		msg := fmt.Sprintf("Item with UID %v do not exist", uid)
		fmt.Println(msg)
	}
}

func (ctrl *cobraCliControl) GetItems(vaultPath, category, title string, trashed bool) {
	ctrl.FirstRun()
	ctrl.CheckForUpdate()
	var vault *domain.Vault

	if vaultPath != "" {
		vault = domain.NewVault(vaultPath)
	} else {
		config := ctrl.configFacade.GetConfig()
		vault = domain.NewVault(config.Vault)
	}

	err := ctrl.vaultFacade.Validate(vault)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Password:")
	password, err := term.ReadPassword(int(syscall.Stdin))
	err = ctrl.vaultFacade.Unlock(vault, string(password))

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	t := table.NewWriter()
	t.SetStyle(table.StyleDouble)
	t.AppendHeader(table.Row{"lp.", "uid", "category", "title"})

	var cat *domain.ItemCategory

	if category != "" {
		category = strings.TrimSpace(category)
		cat, err = domain.ItemCategoryEnum.FromName(category)

		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	title = strings.TrimSpace(title)
	items := ctrl.vaultFacade.GetItems(cat, title, trashed)

	if len(items) > 0 {
		for i, item := range items {
			t.AppendRow(table.Row{i + 1, item.Uid, item.Category.GetName(), item.Title})
		}

		fmt.Println(t.Render())
	} else {
		msg := fmt.Sprintf("No results for search (category: %v, name: %v, trashed: %v)", category, title, trashed)
		fmt.Println(msg)
	}
}

func (ctrl *cobraCliControl) EditItem(vaultPath, uid string, trashed bool) {
	ctrl.FirstRun()
	ctrl.CheckForUpdate()
	var vault *domain.Vault

	if vaultPath != "" {
		vault = domain.NewVault(vaultPath)
	} else {
		config := ctrl.configFacade.GetConfig()
		vault = domain.NewVault(config.Vault)
	}

	err := ctrl.vaultFacade.Validate(vault)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Password:")
	password, err := term.ReadPassword(int(syscall.Stdin))
	err = ctrl.vaultFacade.Unlock(vault, string(password))

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	item := ctrl.vaultFacade.GetItem(uid, trashed)

	if item == nil {
		fmt.Println(fmt.Sprintf("Item with UID %v do not exist", uid))
		return
	}

	payloadMap := payloadFromItem(item)
	edited, err := editJson(payloadMap)

	if err != nil {
		fmt.Println(err)
		return
	}

	payload, err := payloadFromMap(edited)

	if err != nil {
		fmt.Println(err)
		return
	}

	if _, err := ctrl.vaultFacade.UpdateItem(vault, item, payload); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Item updated.")
}

func (ctrl *cobraCliControl) NewItem(vaultPath, category string) {
	ctrl.FirstRun()
	ctrl.CheckForUpdate()
	var vault *domain.Vault

	if vaultPath != "" {
		vault = domain.NewVault(vaultPath)
	} else {
		config := ctrl.configFacade.GetConfig()
		vault = domain.NewVault(config.Vault)
	}

	err := ctrl.vaultFacade.Validate(vault)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Password:")
	password, err := term.ReadPassword(int(syscall.Stdin))
	err = ctrl.vaultFacade.Unlock(vault, string(password))

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	template := newItemTemplate(category)
	edited, err := editJson(template)

	if err != nil {
		fmt.Println(err)
		return
	}

	payload, err := payloadFromMap(edited)

	if err != nil {
		fmt.Println(err)
		return
	}

	item, err := ctrl.vaultFacade.CreateItem(vault, payload)

	if err != nil {
		fmt.Println(err)
		return
	}

	if item != nil {
		fmt.Println(fmt.Sprintf("Item created: %v", item.Uid))
	} else {
		fmt.Println("Item created.")
	}
}

func newItemTemplate(category string) map[string]interface{} {
	if strings.TrimSpace(category) == "" {
		category = domain.ItemCategoryEnum.SecureNote.GetCode()
	}

	overview := make(map[string]interface{})
	overview["title"] = "New Item"
	overview["url"] = ""

	details := make(map[string]interface{})
	details["notesPlain"] = ""
	details["sections"] = make([]interface{}, 0)

	payload := make(map[string]interface{})
	payload["category"] = category
	payload["overview"] = overview
	payload["details"] = details
	payload["trashed"] = false

	return payload
}

func payloadFromItem(item *domain.Item) map[string]interface{} {
	payload := make(map[string]interface{})

	if item.Raw != nil {
		for key, value := range item.Raw {
			if isReservedMetaKey(key) {
				continue
			}

			payload[key] = value
		}
	} else {
		payload["uuid"] = item.Uid
		payload["category"] = item.Category.GetCode()
		payload["created"] = item.Created
		payload["updated"] = item.Updated
		payload["trashed"] = item.Trashed
	}

	overview := item.Overview
	if overview == nil {
		overview = make(map[string]interface{})
	}

	details := item.Details
	if details == nil {
		details = make(map[string]interface{})
	}

	payload["overview"] = overview
	payload["details"] = details

	return payload
}

func payloadFromMap(data map[string]interface{}) (*domain.ItemPayload, error) {
	if data == nil {
		return nil, domain.ErrInvalidPayload
	}

	overview, ok := data["overview"].(map[string]interface{})

	if !ok {
		return nil, domain.ErrInvalidPayload
	}

	details, ok := data["details"].(map[string]interface{})

	if !ok {
		return nil, domain.ErrInvalidPayload
	}

	delete(data, "overview")
	delete(data, "details")
	delete(data, "hmac")
	delete(data, "k")
	delete(data, "d")
	delete(data, "o")

	return &domain.ItemPayload{
		Overview: overview,
		Details:  details,
		Meta:     data,
	}, nil
}

func editJson(data map[string]interface{}) (map[string]interface{}, error) {
	file, err := ioutil.TempFile("", "1pass-edit-*.json")

	if err != nil {
		return nil, err
	}

	defer os.Remove(file.Name())

	body, err := json.MarshalIndent(data, "", "  ")

	if err != nil {
		return nil, err
	}

	if _, err := file.Write(body); err != nil {
		return nil, err
	}

	if err := file.Close(); err != nil {
		return nil, err
	}

	editor := os.Getenv("EDITOR")

	if strings.TrimSpace(editor) == "" {
		editor = "vi"
	}

	cmd := exec.Command("sh", "-c", editor+" "+file.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	updated, err := ioutil.ReadFile(file.Name())

	if err != nil {
		return nil, err
	}

	if len(strings.TrimSpace(string(updated))) == 0 {
		return nil, domain.ErrInvalidPayload
	}

	var result map[string]interface{}

	if err := json.Unmarshal(updated, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func isReservedMetaKey(key string) bool {
	switch key {
	case "d", "o", "k", "hmac":
		return true
	default:
		return false
	}
}

func (ctrl *cobraCliControl) DebugItem(vaultPath, uid string, trashed bool) {
	ctrl.FirstRun()
	ctrl.CheckForUpdate()
	var vault *domain.Vault

	if vaultPath != "" {
		vault = domain.NewVault(vaultPath)
	} else {
		config := ctrl.configFacade.GetConfig()
		vault = domain.NewVault(config.Vault)
	}

	err := ctrl.vaultFacade.Validate(vault)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Password:")
	password, err := term.ReadPassword(int(syscall.Stdin))
	err = ctrl.vaultFacade.Unlock(vault, string(password))

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	item := ctrl.vaultFacade.GetItem(uid, trashed)

	if item == nil {
		fmt.Println(fmt.Sprintf("Item with UID %v do not exist", uid))
		return
	}

	fmt.Println("=== Debug Item ===")
	fmt.Println(fmt.Sprintf("UID: %v", item.Uid))
	fmt.Println(fmt.Sprintf("Category: %v", item.Category.GetName()))
	fmt.Println(fmt.Sprintf("Trashed: %v", item.Trashed))

	overview, _ := json.MarshalIndent(item.Overview, "", "  ")
	details, _ := json.MarshalIndent(item.Details, "", "  ")

	fmt.Println("\n--- Overview JSON ---")
	fmt.Println(string(overview))
	fmt.Println("\n--- Details JSON ---")
	fmt.Println(string(details))

	fmt.Println("\n--- Parsed Sections ---")
	sections := buildDebugSections(item.Sections)
	sectionsJson, _ := json.MarshalIndent(sections, "", "  ")
	fmt.Println(string(sectionsJson))

	fmt.Println("\n--- Notes ---")
	fmt.Println(item.Notes)

	fmt.Println("\n--- Raw Meta ---")
	meta := rawMeta(item.Raw)
	metaJson, _ := json.MarshalIndent(meta, "", "  ")
	fmt.Println(string(metaJson))
}

func (ctrl *cobraCliControl) Update() {
	fmt.Println("Checking for 1pass application updates...")

	info, err := ctrl.updateFacade.CheckForUpdate(true)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	msg := fmt.Sprintf("New version of 1pass is available (%v).\n\n%v\n", info.Version, info.Changelog)
	fmt.Println(msg)
	var permissionVal string
	fmt.Print(fmt.Sprintf("Do you want to update now? (%v) [y - for yes/n - for no]: ", domain.LogicValEnum.No.GetName()))
	fmt.Scanln(&permissionVal)
	permission, err := domain.LogicValEnum.FromName(permissionVal)

	if err != nil || permission == domain.LogicValEnum.No {
		fmt.Println("Aborting update...")
		os.Exit(1)
	}

	msg = fmt.Sprintf("Updating 1pass application from version %v to %v...", domain.Version, info.Version)
	fmt.Println(msg)

	stageInfo := func(step int) {
		switch step {
		case 1:
			fmt.Println("  Creating update cache...")

		case 2:
			fmt.Println("  Downloading new version...")

		case 3:
			fmt.Println("  Downloading checksums...")

		case 4:
			fmt.Println("  Extracting downloaded files...")

		case 5:
			fmt.Println("  Validating checksums...")

		case 6:
			fmt.Println("  Replacing binary...")

		case 7:
			fmt.Println("  Cleaning update cache...")

		default:
			fmt.Println("  Unknown update stage")
		}
	}

	if err := ctrl.updateFacade.Update(stageInfo); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(fmt.Sprintf("1pass application updated to version %v", info.Version))
}

func buildDebugSections(sections []*domain.ItemSection) []map[string]interface{} {
	result := make([]map[string]interface{}, 0)

	for _, section := range sections {
		entry := make(map[string]interface{})
		entry["title"] = section.Title

		fields := make([]map[string]interface{}, 0)

		for _, field := range section.Fields {
			fields = append(fields, map[string]interface{}{
				"name":  field.Name,
				"value": field.Value,
			})
		}

		entry["fields"] = fields
		result = append(result, entry)
	}

	return result
}

func rawMeta(raw map[string]interface{}) map[string]interface{} {
	if raw == nil {
		return nil
	}

	meta := make(map[string]interface{})

	for key, value := range raw {
		if key == "d" || key == "o" || key == "k" || key == "hmac" {
			continue
		}

		meta[key] = value
	}

	return meta
}
