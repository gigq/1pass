// Widget for editing items.
//
// @author TSS

package gui

import (
	"fmt"
	"strings"

	"github.com/jroimartin/gocui"
	"github.com/mashmb/1pass/1pass-core/core/domain"
	"github.com/mashmb/1pass/1pass-core/port/in"
)

const (
	editHelp string = `Save: CTRL+S
Next field: TAB
Prev field: CTRL+P
Cancel: ESC or CTRL+Q`
)

type formFieldKind int

const (
	fieldTitle formFieldKind = iota
	fieldURL
	fieldNotes
	fieldValue
)

type formField struct {
	kind     formFieldKind
	label    string
	value    string
	target   map[string]interface{}
	valueKey string
}

type editWidget struct {
	name       string
	title      string
	parent     string
	fieldView  string
	valueView  string
	helpWidget *helpWidget
	errDialog  *errorDialog
	guiControl in.GuiControl
	vault      *domain.Vault
	item       *domain.Item
	refresh    func(ui *gocui.Gui) error
	fields     []formField
	current    int
	overview   map[string]interface{}
	details    map[string]interface{}
	meta       map[string]interface{}
	prevCursor bool
}

func newEditWidget(parent string, helpWidget *helpWidget, vault *domain.Vault, guiControl in.GuiControl, refresh func(ui *gocui.Gui) error) *editWidget {
	widget := &editWidget{
		name:       "editWidget",
		parent:     parent,
		fieldView:  "editFields",
		valueView:  "editValue",
		helpWidget: helpWidget,
		guiControl: guiControl,
		vault:      vault,
		refresh:    refresh,
	}

	widget.errDialog = newErrorDialog("editErrDialog", widget.closeError)

	return widget
}

func (ew *editWidget) Keybindings(ui *gocui.Gui) error {
	if err := ew.errDialog.Keybindings(ui); err != nil {
		return err
	}

	views := []string{ew.fieldView, ew.valueView}

	for _, view := range views {
		if err := ui.SetKeybinding(view, gocui.KeyCtrlS, gocui.ModNone, ew.save); err != nil {
			return err
		}

		if err := ui.SetKeybinding(view, gocui.KeyEsc, gocui.ModNone, ew.cancel); err != nil {
			return err
		}

		if err := ui.SetKeybinding(view, gocui.KeyCtrlQ, gocui.ModNone, ew.cancel); err != nil {
			return err
		}

		if err := ui.SetKeybinding(view, gocui.KeyCtrlC, gocui.ModNone, ew.cancel); err != nil {
			return err
		}
	}

	if err := ui.SetKeybinding(ew.valueView, gocui.KeyTab, gocui.ModNone, ew.nextField); err != nil {
		return err
	}

	if err := ui.SetKeybinding(ew.valueView, gocui.KeyCtrlN, gocui.ModNone, ew.nextField); err != nil {
		return err
	}

	if err := ui.SetKeybinding(ew.valueView, gocui.KeyCtrlP, gocui.ModNone, ew.prevField); err != nil {
		return err
	}

	if err := ui.SetKeybinding(ew.fieldView, 'j', gocui.ModNone, ew.nextField); err != nil {
		return err
	}

	if err := ui.SetKeybinding(ew.fieldView, gocui.KeyArrowDown, gocui.ModNone, ew.nextField); err != nil {
		return err
	}

	if err := ui.SetKeybinding(ew.fieldView, 'k', gocui.ModNone, ew.prevField); err != nil {
		return err
	}

	if err := ui.SetKeybinding(ew.fieldView, gocui.KeyArrowUp, gocui.ModNone, ew.prevField); err != nil {
		return err
	}

	if err := ui.SetKeybinding(ew.fieldView, gocui.KeyEnter, gocui.ModNone, ew.focusValue); err != nil {
		return err
	}

	return nil
}

func (ew *editWidget) Open(ui *gocui.Gui, item *domain.Item, payload map[string]interface{}) error {
	maxX, maxY := ui.Size()
	ew.item = item

	if item != nil {
		ew.title = "Edit Item"
	} else {
		ew.title = "New Item"
	}

	ew.prevCursor = ui.Cursor
	ui.Cursor = true

	if err := ew.loadPayload(payload); err != nil {
		return err
	}

	leftWidth := int(0.35 * float32(maxX-1))
	if leftWidth < 24 {
		leftWidth = 24
	}
	if leftWidth > maxX-20 {
		leftWidth = maxX - 20
	}

	fieldsView, err := ui.SetView(ew.fieldView, 0, 3, leftWidth, maxY-4)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	fieldsView.Title = "Fields"
	fieldsView.Editable = false
	fieldsView.Wrap = false
	fieldsView.Clear()

	valueView, err := ui.SetView(ew.valueView, leftWidth+1, 3, maxX-1, maxY-4)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}

	valueView.Title = "Value"
	valueView.Editable = true
	valueView.Wrap = true
	valueView.Clear()
	_ = valueView.SetOrigin(0, 0)
	_ = valueView.SetCursor(0, 0)

	if err := ew.updateFieldList(ui); err != nil {
		return err
	}

	if err := ew.loadCurrentValue(ui); err != nil {
		return err
	}

	if _, err := ui.SetCurrentView(ew.valueView); err != nil {
		return err
	}

	ew.helpWidget.help = editHelp
	if err := ew.helpWidget.update(ui); err != nil {
		return err
	}

	return nil
}

func (ew *editWidget) cancel(ui *gocui.Gui, view *gocui.View) error {
	_ = ui.DeleteView(ew.valueView)
	_ = ui.DeleteView(ew.fieldView)
	ui.Cursor = ew.prevCursor

	if _, err := ui.SetCurrentView(ew.parent); err != nil {
		return err
	}

	ew.helpWidget.help = itemsHelp
	if err := ew.helpWidget.update(ui); err != nil {
		return err
	}

	return nil
}

func (ew *editWidget) save(ui *gocui.Gui, view *gocui.View) error {
	if err := ew.storeCurrentValue(ui); err != nil {
		return err
	}

	ew.applyFields()
	payload := &domain.ItemPayload{
		Overview: ew.overview,
		Details:  ew.details,
		Meta:     ew.meta,
	}

	if ew.item != nil {
		if _, err := ew.guiControl.UpdateItem(ew.vault, ew.item, payload); err != nil {
			ew.showError(ui, err)
			return nil
		}
	} else {
		if _, err := ew.guiControl.CreateItem(ew.vault, payload); err != nil {
			ew.showError(ui, err)
			return nil
		}
	}

	if err := ew.cancel(ui, view); err != nil {
		return err
	}

	if ew.refresh != nil {
		if err := ew.refresh(ui); err != nil {
			return err
		}
	}

	return nil
}

func (ew *editWidget) nextField(ui *gocui.Gui, view *gocui.View) error {
	if len(ew.fields) == 0 {
		return nil
	}

	if err := ew.storeCurrentValue(ui); err != nil {
		return err
	}

	ew.current = (ew.current + 1) % len(ew.fields)

	if err := ew.updateFieldList(ui); err != nil {
		return err
	}

	return ew.loadCurrentValue(ui)
}

func (ew *editWidget) prevField(ui *gocui.Gui, view *gocui.View) error {
	if len(ew.fields) == 0 {
		return nil
	}

	if err := ew.storeCurrentValue(ui); err != nil {
		return err
	}

	ew.current--
	if ew.current < 0 {
		ew.current = len(ew.fields) - 1
	}

	if err := ew.updateFieldList(ui); err != nil {
		return err
	}

	return ew.loadCurrentValue(ui)
}

func (ew *editWidget) focusValue(ui *gocui.Gui, view *gocui.View) error {
	if _, err := ui.SetCurrentView(ew.valueView); err != nil {
		return err
	}

	return nil
}

func (ew *editWidget) storeCurrentValue(ui *gocui.Gui) error {
	if len(ew.fields) == 0 || ew.current < 0 || ew.current >= len(ew.fields) {
		return nil
	}

	view, err := ui.View(ew.valueView)
	if err != nil {
		return err
	}

	value := strings.TrimRight(view.Buffer(), "\n")
	ew.fields[ew.current].value = value

	return nil
}

func (ew *editWidget) loadCurrentValue(ui *gocui.Gui) error {
	if len(ew.fields) == 0 || ew.current < 0 || ew.current >= len(ew.fields) {
		return nil
	}

	view, err := ui.View(ew.valueView)
	if err != nil {
		return err
	}

	view.Clear()
	field := ew.fields[ew.current]
	view.Title = fmt.Sprintf("Value: %s", field.label)
	fmt.Fprint(view, field.value)
	_ = view.SetOrigin(0, 0)
	_ = view.SetCursor(0, 0)

	return nil
}

func (ew *editWidget) updateFieldList(ui *gocui.Gui) error {
	view, err := ui.View(ew.fieldView)
	if err != nil {
		return err
	}

	view.Clear()

	for idx, field := range ew.fields {
		prefix := "  "
		if idx == ew.current {
			prefix = "> "
		}
		fmt.Fprintf(view, "%s%s\n", prefix, field.label)
	}

	return nil
}

func (ew *editWidget) applyFields() {
	for _, field := range ew.fields {
		switch field.kind {
		case fieldTitle:
			ew.overview["title"] = field.value
		case fieldURL:
			ew.overview["url"] = field.value
		case fieldNotes:
			ew.details["notesPlain"] = field.value
		case fieldValue:
			if field.target != nil && field.valueKey != "" {
				field.target[field.valueKey] = field.value
			}
		}
	}
}

func (ew *editWidget) showError(ui *gocui.Gui, err error) {
	ew.errDialog.err = err
	_ = ew.errDialog.Layout(ui)
}

func (ew *editWidget) closeError(ui *gocui.Gui, view *gocui.View) error {
	if err := ui.DeleteView(ew.errDialog.name); err != nil {
		return err
	}

	if _, err := ui.SetCurrentView(ew.valueView); err != nil {
		return err
	}

	return nil
}

func (ew *editWidget) loadPayload(payload map[string]interface{}) error {
	if payload == nil {
		return domain.ErrInvalidPayload
	}

	overview, ok := payload["overview"].(map[string]interface{})
	if !ok || overview == nil {
		overview = make(map[string]interface{})
	}

	details, ok := payload["details"].(map[string]interface{})
	if !ok || details == nil {
		details = make(map[string]interface{})
	}

	meta := make(map[string]interface{})
	for key, value := range payload {
		if key == "overview" || key == "details" {
			continue
		}
		if isReservedMetaKey(key) {
			continue
		}
		meta[key] = value
	}

	ew.overview = overview
	ew.details = details
	ew.meta = meta
	ew.fields = buildFormFields(overview, details)
	ew.current = 0

	return nil
}

func buildFormFields(overview, details map[string]interface{}) []formField {
	fields := []formField{
		{kind: fieldTitle, label: "Title", value: stringValue(overview["title"])},
		{kind: fieldURL, label: "URL", value: stringValue(overview["url"])},
		{kind: fieldNotes, label: "Notes", value: stringValue(details["notesPlain"])},
	}

	if sectionsRaw, ok := details["sections"].([]interface{}); ok {
		for _, sectionRaw := range sectionsRaw {
			section, ok := sectionRaw.(map[string]interface{})
			if !ok {
				continue
			}

			sectionTitle := stringValue(section["title"])
			fieldsRaw, ok := section["fields"].([]interface{})
			if !ok {
				continue
			}

			for _, fieldRaw := range fieldsRaw {
				fieldMap, ok := fieldRaw.(map[string]interface{})
				if !ok {
					continue
				}

				fieldTitle := stringValue(fieldMap["t"])
				label := strings.Title(fieldTitle)
				if sectionTitle != "" {
					label = fmt.Sprintf("%s / %s", strings.Title(sectionTitle), label)
				}

				fields = append(fields, formField{
					kind:     fieldValue,
					label:    label,
					value:    stringValue(fieldMap["v"]),
					target:   fieldMap,
					valueKey: "v",
				})
			}
		}

		return fields
	}

	if fieldsRaw, ok := details["fields"].([]interface{}); ok {
		for _, fieldRaw := range fieldsRaw {
			fieldMap, ok := fieldRaw.(map[string]interface{})
			if !ok {
				continue
			}

			label := strings.Title(stringValue(fieldMap["name"]))
			fields = append(fields, formField{
				kind:     fieldValue,
				label:    label,
				value:    stringValue(fieldMap["value"]),
				target:   fieldMap,
				valueKey: "value",
			})
		}
	}

	return fields
}

func stringValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func newItemTemplate(category *domain.ItemCategory) map[string]interface{} {
	code := domain.ItemCategoryEnum.SecureNote.GetCode()
	if category != nil {
		code = category.GetCode()
	}

	overview := make(map[string]interface{})
	overview["title"] = "New Item"
	overview["url"] = ""

	details := make(map[string]interface{})
	details["notesPlain"] = ""
	details["sections"] = make([]interface{}, 0)

	payload := make(map[string]interface{})
	payload["category"] = code
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

func isReservedMetaKey(key string) bool {
	switch key {
	case "d", "o", "k", "hmac":
		return true
	default:
		return false
	}
}
