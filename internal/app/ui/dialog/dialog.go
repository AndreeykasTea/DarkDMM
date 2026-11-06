package dialog

import (
	"sync"

	"github.com/SpaiR/imgui-go"
	"github.com/rs/zerolog/log"
)

type Type interface {
	Name() string
	Process()
	HasCloseButton() bool
}

const popupFlags = imgui.WindowFlagsAlwaysAutoResize | imgui.WindowFlagsNoSavedSettings

var (
	openedMtx sync.Mutex
	opened    []Type
)

func snapshot() []Type {
	openedMtx.Lock()
	defer openedMtx.Unlock()
	cpy := make([]Type, len(opened))
	copy(cpy, opened)
	return cpy
}

func Process() {
	var closedDialogs []Type
	for _, dialog := range snapshot() {
		if !imgui.IsPopupOpen(dialog.Name()) {
			imgui.OpenPopup(dialog.Name())
		}

		var isOpen bool
		if dialog.HasCloseButton() {
			open := true
			isOpen = imgui.BeginPopupModalV(dialog.Name(), &open, popupFlags)
		} else {
			isOpen = imgui.BeginPopupModalV(dialog.Name(), nil, popupFlags)
		}

		if isOpen {
			dialog.Process()
			imgui.EndPopup()
		}

		if !imgui.IsPopupOpen(dialog.Name()) {
			closedDialogs = append(closedDialogs, dialog)
		}
	}

	for _, dialog := range closedDialogs {
		Close(dialog)
	}
}

// Open opens the application dialog. Safe to call from any goroutine.
func Open(t Type) {
	log.Print("opening dialog:", t.Name())
	openedMtx.Lock()
	opened = append(opened, t)
	openedMtx.Unlock()
}

// Close closed the application dialog. Safe to call from any goroutine.
func Close(dialog Type) {
	log.Print("closing dialog:", dialog.Name())
	openedMtx.Lock()
	defer openedMtx.Unlock()
	for idx, t := range opened {
		if dialog.Name() == t.Name() {
			log.Print("dialog closed:", dialog.Name())
			opened = append(opened[:idx], opened[idx+1:]...)
			return
		}
	}
}

// HasOpened returns true if there is at least one opened dialog.
func HasOpened() bool {
	openedMtx.Lock()
	defer openedMtx.Unlock()
	return len(opened) > 0
}
