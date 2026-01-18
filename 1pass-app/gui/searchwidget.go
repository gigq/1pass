// Search widget for filtering items.
//
// @author TSS

package gui

import (
	"strings"

	"github.com/jroimartin/gocui"
)

type searchWidget struct {
	name         string
	title        string
	query        string
	previousView string
	onChange     func(ui *gocui.Gui, query string) error
}

func newSearchWidget(onChange func(ui *gocui.Gui, query string) error) *searchWidget {
	return &searchWidget{
		name:     "searchWidget",
		title:    "Search",
		onChange: onChange,
	}
}

func (sw *searchWidget) focus(ui *gocui.Gui, _ *gocui.View) error {
	if current := ui.CurrentView(); current != nil && current.Name() != sw.name {
		sw.previousView = current.Name()
	}

	if _, err := ui.SetCurrentView(sw.name); err != nil {
		return err
	}

	return nil
}

func (sw *searchWidget) blur(ui *gocui.Gui, _ *gocui.View) error {
	if sw.previousView != "" {
		if _, err := ui.View(sw.previousView); err == nil {
			_, err = ui.SetCurrentView(sw.previousView)
			return err
		}
	}

	if _, err := ui.View("itemsWidget"); err == nil {
		_, err = ui.SetCurrentView("itemsWidget")
		return err
	}

	if _, err := ui.SetCurrentView("1pass"); err != nil {
		return err
	}

	return nil
}

func (sw *searchWidget) Keybindings(ui *gocui.Gui) error {
	if err := ui.SetKeybinding("", '/', gocui.ModNone, sw.focus); err != nil {
		return err
	}

	if err := ui.SetKeybinding("", gocui.KeyCtrlF, gocui.ModNone, sw.focus); err != nil {
		return err
	}

	if err := ui.SetKeybinding(sw.name, gocui.KeyEsc, gocui.ModNone, sw.blur); err != nil {
		return err
	}

	if err := ui.SetKeybinding(sw.name, gocui.KeyEnter, gocui.ModNone, sw.blur); err != nil {
		return err
	}

	if err := ui.SetKeybinding(sw.name, gocui.KeyTab, gocui.ModNone, sw.blur); err != nil {
		return err
	}

	return nil
}

func (sw *searchWidget) Layout(ui *gocui.Gui) error {
	maxX, _ := ui.Size()

	if view, err := ui.SetView(sw.name, 0, 0, maxX-1, 2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}

		view.Title = sw.title
		view.Editable = true
		view.Editor = gocui.EditorFunc(func(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
			gocui.DefaultEditor.Edit(v, key, ch, mod)
			query := strings.TrimRight(v.Buffer(), "\n")
			sw.query = query

			if sw.onChange != nil {
				_ = sw.onChange(ui, sw.query)
			}
		})
	}

	return nil
}
