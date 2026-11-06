package prefs

const (
	SaveFormatInitial = "Initial"
	SaveFormatTGM     = "TGM"
	SaveFormatDMM     = "DMM"

	SaveFormatHelp = `Initial - карта сохраняется в том формате, в котором была загружена
TGM - формат от TG: карта читается легче и меньше конфликтов при слиянии
DMM - стандартный формат редактора карт DM
`
)

var SaveFormats = []string{
	SaveFormatInitial,
	SaveFormatTGM,
	SaveFormatDMM,
}

const (
	CodeEditorVSC      = "Visual Studio Code"
	CodeEditorVSCodium = "VSCodium"
	CodeEditorDM       = "Dreammaker"
	CodeEditorNPP      = "Notepad++"
	CodeEditorDefault  = "Default App"

	CodeEditorVSCActual      = "code"
	CodeEditorVSCodiumActual = "codium"
	CodeEditorDMActual       = "dreammaker"
	CodeEditorNPPActual      = "notepad++"

	CodeEditorHelp = `Программа должна быть в системном PATH, чтобы работало «Перейти к определению».
Default App и Dreammaker открывают только нужный файл, без перехода к строке.`
)

var CodeEditors = []string{
	CodeEditorVSC,
	CodeEditorVSCodium,
	CodeEditorDM,
	CodeEditorNPP,
	CodeEditorDefault,
}

const (
	SaveNudgeModePixel    = "pixel_x/pixel_y"
	SaveNudgeModeStep     = "step_x/step_y"
	SaveNudgeModePixelAlt = "pixel_w/pixel_z"
)

var SaveNudgeModes = []string{
	SaveNudgeModePixel,
	SaveNudgeModeStep,
	SaveNudgeModePixelAlt,
}
