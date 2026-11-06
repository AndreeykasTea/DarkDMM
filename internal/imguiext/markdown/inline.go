package markdown

import (
	"strings"

	"sdmm/internal/imguiext/style"

	"github.com/SpaiR/imgui-go"
)

type spanKind int

const (
	spanPlain spanKind = iota
	spanBold
	spanCode
)

// span is a part of a text with the same formatting.
type span struct {
	kind spanKind
	text string
}

// parseInline splits a line by **bold** and `code` markers. A marker without a pair stays as a usual text.
func parseInline(text string) []span {
	var res []span
	var plain strings.Builder

	flush := func() {
		if plain.Len() > 0 {
			res = append(res, span{spanPlain, plain.String()})
			plain.Reset()
		}
	}

	for len(text) > 0 {
		marker, kind := "", spanPlain
		switch {
		case strings.HasPrefix(text, "**"):
			marker, kind = "**", spanBold
		case strings.HasPrefix(text, "`"):
			marker, kind = "`", spanCode
		}

		if marker != "" {
			// An empty pair ("****") is not a formatting.
			if end := strings.Index(text[len(marker):], marker); end > 0 {
				flush()
				res = append(res, span{kind, text[len(marker) : len(marker)+end]})
				text = text[len(marker)+end+len(marker):]
				continue
			}
		}

		// Text is split only by ASCII markers, so it's safe for UTF-8: markers are never a part of a multibyte rune.
		plain.WriteByte(text[0])
		text = text[1:]
	}
	flush()

	return res
}

// layoutSpans splits spans by lines, which fit the width. Words are never broken, a word
// which is wider than the width takes its own line. The measure returns a width of a text.
func layoutSpans(spans []span, width float32, measure func(string) float32) [][]span {
	lines := [][]span{nil}
	var lineWidth float32

	add := func(kind spanKind, text string) {
		line := lines[len(lines)-1]
		if n := len(line); n > 0 && line[n-1].kind == kind {
			line[n-1].text += text
		} else {
			lines[len(lines)-1] = append(line, span{kind, text})
		}
		lineWidth += measure(text)
	}

	// Spaces at the end of a line are not needed.
	trimLine := func() {
		line := lines[len(lines)-1]
		for n := len(line); n > 0; n-- {
			line[n-1].text = strings.TrimRight(line[n-1].text, " ")
			if line[n-1].text != "" {
				break
			}
			line = line[:n-1]
		}
		lines[len(lines)-1] = line
	}

	newLine := func() {
		trimLine()
		lines = append(lines, nil)
		lineWidth = 0
	}

	for _, s := range spans {
		for partIdx, part := range strings.Split(s.text, "\n") {
			if partIdx > 0 {
				newLine()
			}
			for _, word := range strings.SplitAfter(part, " ") {
				if word == "" {
					continue
				}
				// Only spaces never move to the next line.
				if visible := strings.TrimRight(word, " "); visible != "" && lineWidth > 0 && lineWidth+measure(visible) > width {
					newLine()
				}
				add(s.kind, word)
			}
		}
	}

	trimLine()
	return lines
}

var colorCode = imgui.Vec4{X: .6, Y: .8, Z: 1, W: 1}

// showInline shows a wrapped text with inline formatting.
func showInline(text string) {
	spans := parseInline(text)
	// The usual text is wrapped by ImGui itself, it knows how to break long words.
	if len(spans) == 0 || (len(spans) == 1 && spans[0].kind == spanPlain) {
		imgui.TextWrapped(text)
		return
	}

	measure := func(s string) float32 {
		return imgui.CalcTextSize(s, false, 0).X
	}
	for _, line := range layoutSpans(spans, imgui.ContentRegionAvail().X, measure) {
		for idx, s := range line {
			if idx > 0 {
				imgui.SameLineV(0, 0)
			}
			switch s.kind {
			case spanBold:
				imgui.TextColored(style.ColorGoldLighter, s.text)
			case spanCode:
				imgui.TextColored(colorCode, s.text)
			default:
				imgui.Text(s.text)
			}
		}
		if len(line) == 0 {
			imgui.NewLine()
		}
	}
}
