package pmap

import (
	"fmt"
	"path/filepath"
	"time"

	"sdmm/internal/app/prefs"

	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dmlint"
	"sdmm/internal/imguiext/style"

	"github.com/SpaiR/imgui-go"
	"github.com/rs/zerolog/log"
)

// LintOnOpen runs the map check when the map is opened.
var LintOnOpen = true

// LintExtra enables heuristic checks, which are not done by the CI of the code base.
var LintExtra = false

// Maplint rules are loaded once per code base.
var (
	maplintRoot  string
	maplintRules *dmlint.MaplintRules
	ciRules      *dmlint.CIRules
)

func (p *PaneMap) lintOptions() dmlint.Options {
	env := p.app.LoadedEnvironment()
	if maplintRoot != env.RootDir {
		maplintRoot = env.RootDir
		ciRules = dmlint.LoadCIRules(env.RootDir)
		maplintRules = ciRules.Maplint
	}

	mapFile := p.dmm.Path.Readable
	saveFormat := p.app.Prefs().Editor.SaveFormat
	isTGMFile := dmlint.IsTGMFile(p.dmm.Path.Absolute)
	savesAsTGM := saveFormat == prefs.SaveFormatTGM || (saveFormat == prefs.SaveFormatInitial && isTGMFile)
	// A code base could want the TGM format, then the rewrite is necessary and the CI check reports about it.
	rewrites := !ciRules.TextChecks && ((saveFormat == prefs.SaveFormatTGM && !isTGMFile) || (saveFormat == prefs.SaveFormatDMM && isTGMFile))

	return dmlint.Options{
		Maplint:    maplintRules,
		TextChecks: ciRules.TextChecks,
		RequireTGM: ciRules.TextChecks,
		Greps:      ciRules.Greps,
		UnitTests:  ciRules.UnitTests,

		RewritesFormat: rewrites,
		MapFile:        filepath.ToSlash(mapFile),
		SavesAsTGM:     savesAsTGM,
		Extra:          LintExtra,
	}
}

const lintMaxShownPerCategory = 300

type lintWindow struct {
	open    bool
	checked bool // The check on the map open is done.

	issues  []dmlint.Issue
	elapsed time.Duration

	showErrors, showWarnings, showInfos bool

	// afterSave is true when the window was opened by the check after the save.
	afterSave bool
}

// OpenLintWindow runs the check and shows the result.
func (p *PaneMap) OpenLintWindow() {
	p.lintWin.afterSave = false
	p.runLint()
	p.lintWin.open = true
}

func (p *PaneMap) runLint() {
	lw := &p.lintWin
	start := time.Now()
	lw.issues = dmlint.Check(p.app.LoadedEnvironment(), p.dmm, p.lintOptions())
	lw.elapsed = time.Since(start)
	lw.checked = true
	errors, warnings, infos := dmlint.Summary(lw.issues)
	log.Printf("map check [%s]: errors [%d], warnings [%d], infos [%d] in [%d] ms",
		p.dmm.Name, errors, warnings, infos, lw.elapsed.Milliseconds())
}

func (p *PaneMap) processLintWindow() {
	lw := &p.lintWin

	if !lw.checked && LintOnOpen && p.app.LoadedEnvironment() != nil {
		lw.showErrors, lw.showWarnings = true, true
		p.runLint()
		// The window is opened automatically only for real errors and before a rewrite of the whole map.
		if errors, _, _ := dmlint.Summary(lw.issues); errors > 0 || hasRewriteWarning(lw.issues) {
			lw.open = true
		}
	}

	if !lw.open {
		return
	}

	imgui.SetNextWindowSizeV(imgui.Vec2{X: window.PointSize() * 560, Y: window.PointSize() * 420}, imgui.ConditionFirstUseEver)
	open := true
	if imgui.BeginV("Проверка карты: "+p.dmm.Name+"###lint_window_"+p.dmm.Name, &open, imgui.WindowFlagsNoDocking) {
		p.showLintContent()
	}
	imgui.End()
	if !open {
		lw.open = false
		lw.afterSave = false
	}
}

func hasRewriteWarning(issues []dmlint.Issue) bool {
	for _, issue := range issues {
		if issue.Category == dmlint.CatSaveFormat {
			return true
		}
	}
	return false
}

func (p *PaneMap) showLintContent() {
	lw := &p.lintWin
	errors, warnings, infos := dmlint.Summary(lw.issues)

	if imgui.Button("Проверить снова") {
		p.runLint()
	}
	imgui.SameLine()
	imgui.Checkbox("Ошибки ("+fmt.Sprint(errors)+")##lint_e", &lw.showErrors)
	imgui.SameLine()
	imgui.Checkbox("Предупреждения ("+fmt.Sprint(warnings)+")##lint_w", &lw.showWarnings)
	imgui.SameLine()
	imgui.Checkbox("Заметки ("+fmt.Sprint(infos)+")##lint_i", &lw.showInfos)

	imgui.Checkbox("Проверять при открытии карты##lint_on_open", &LintOnOpen)
	imgui.SameLine()
	imgui.Checkbox("После сохранения##lint_on_save", &LintOnSave)
	imgui.SameLine()
	imgui.Checkbox("Доп. проверки (не из CI)##lint_extra", &LintExtra)
	if imgui.IsItemHovered() {
		imgui.SetTooltip("Обрывы кабелей и труб, APC в зонах. CI этого не проверяет, поэтому по умолчанию выключено.")
	}
	if maplintRules != nil {
		imgui.TextDisabled(fmt.Sprintf("Правила maplint из билда: %d", len(maplintRules.Lints)))
		for _, e := range maplintRules.Errors {
			imgui.TextColored(style.ColorRed, "Не удалось прочитать правило: "+e)
		}
	}
	if ciRules != nil && len(ciRules.Greps) > 0 {
		imgui.TextDisabled(fmt.Sprintf("Проверки из test/run-test.sh: %d", len(ciRules.Greps)))
	}
	if ciRules != nil && ciRules.UnitTests {
		imgui.TextDisabled("Юнит-тесты карт из code/unit_tests: включены")
		if imgui.IsItemHovered() {
			imgui.SetTooltip("Их проверки перенесены в редактор как предупреждения: сохранению и окну при открытии карты они не мешают.")
		}
	}
	if maplintRules == nil && (ciRules == nil || (len(ciRules.Greps) == 0 && !ciRules.UnitTests)) {
		imgui.TextDisabled("Проверки CI в билде не найдены, используются только встроенные")
	}
	imgui.TextDisabled(fmt.Sprintf("Проверено за %d мс", lw.elapsed.Milliseconds()))

	imgui.Separator()

	if lw.afterSave && len(lw.issues) > 0 {
		imgui.TextColored(style.ColorRedLighter, "Карта сохранена, но CI найдёт в ней ошибки:")
	}

	if len(lw.issues) == 0 {
		imgui.TextColored(style.ColorGreen3, "Проблем не найдено")
		return
	}

	// Group issues by category, keeping the sort order.
	var categories []dmlint.Category
	byCategory := make(map[dmlint.Category][]int)
	for idx, issue := range lw.issues {
		if !p.lintVisible(issue) {
			continue
		}
		if _, ok := byCategory[issue.Category]; !ok {
			categories = append(categories, issue.Category)
		}
		byCategory[issue.Category] = append(byCategory[issue.Category], idx)
	}

	imgui.BeginChildV("lint_list", imgui.Vec2{}, false, 0)
	for _, cat := range categories {
		indexes := byCategory[cat]
		fixable := 0
		for _, idx := range indexes {
			if lw.issues[idx].Fix != nil {
				fixable++
			}
		}

		open := imgui.CollapsingHeaderV(fmt.Sprintf("%s (%d)###lint_cat_%s", cat, len(indexes), cat), imgui.TreeNodeFlagsDefaultOpen)
		if fixable > 0 {
			imgui.SameLine()
			if imgui.SmallButton(fmt.Sprintf("Исправить всё (%d)##lint_fix_all_%s", fixable, cat)) {
				p.fixLintIssues(indexes)
				break // Issues are recomputed, the list is outdated.
			}
		}
		if !open {
			continue
		}

		for n, idx := range indexes {
			if n >= lintMaxShownPerCategory {
				imgui.TextDisabled(fmt.Sprintf("...и ещё %d", len(indexes)-n))
				break
			}
			issue := lw.issues[idx]
			imgui.PushID(fmt.Sprint("lint_issue_", idx))

			color := style.ColorWhite
			switch issue.Severity {
			case dmlint.SeverityError:
				color = style.ColorRedLighter
			case dmlint.SeverityWarning:
				color = style.ColorGold
			}
			if imgui.SmallButton(fmt.Sprintf("%d,%d,%d", issue.Coord.X, issue.Coord.Y, issue.Coord.Z)) {
				p.editor.FocusCameraOnPosition(issue.Coord)
			}
			if imgui.IsItemHovered() {
				imgui.SetTooltip("Показать на карте")
			}
			imgui.SameLine()
			if issue.Fix != nil {
				if imgui.SmallButton("Исправить") {
					p.fixLintIssues([]int{idx})
					imgui.PopID()
					break
				}
				imgui.SameLine()
			}
			imgui.TextColored(color, issue.Message)
			if issue.Help != "" && imgui.IsItemHovered() {
				imgui.SetTooltip(issue.Help)
			}

			imgui.PopID()
		}
	}
	imgui.EndChild()
}

func (p *PaneMap) lintVisible(issue dmlint.Issue) bool {
	lw := &p.lintWin
	switch issue.Severity {
	case dmlint.SeverityError:
		return lw.showErrors
	case dmlint.SeverityWarning:
		return lw.showWarnings
	}
	return lw.showInfos
}

func (p *PaneMap) fixLintIssues(indexes []int) {
	fixed := 0
	for _, idx := range indexes {
		if fix := p.lintWin.issues[idx].Fix; fix != nil && fix() {
			fixed++
		}
	}
	if fixed > 0 {
		p.editor.CommitChanges(fmt.Sprintf("Проверка карты: исправлено %d", fixed))
		p.app.SyncPrefabs()
		p.app.SyncVarEditor()
	}
	p.runLint()
}

// LintOnSave runs the map check after the map is saved.
var LintOnSave = true

// LintAfterSave checks the map after the save and shows the window if there are errors.
func (p *PaneMap) LintAfterSave() {
	if !LintOnSave || p.app.LoadedEnvironment() == nil {
		return
	}
	p.runLint()
	if errors, _, _ := dmlint.Summary(p.lintWin.issues); errors > 0 {
		p.lintWin.showErrors = true
		p.lintWin.open = true
		p.lintWin.afterSave = true
	}
}
