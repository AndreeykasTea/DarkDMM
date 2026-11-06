package prefs

import "sdmm/internal/app/render/lighting"

type Prefs struct {
	Editor      Editor
	Controls    Controls
	Interface   Interface
	Application Application
	Lighting    lighting.Settings
}

type Interface struct {
	Scale int
	Fps   int
}

type Controls struct {
	AltScrollBehaviour   bool
	QuickEditContextMenu bool
	QuickEditMapPane     bool
}

type Editor struct {
	SaveFormat        string
	CodeEditor        string
	NudgeMode         string
	SanitizeVariables bool
	// AutosaveMinutes is an interval of the autosave. 0 disables it.
	AutosaveMinutes int
}

type Application struct {
	CheckForUpdates bool
	AutoUpdate      bool
}
