package markdown

import (
	"reflect"
	"testing"
	"unicode/utf8"
)

func TestParseInline(t *testing.T) {
	cases := []struct {
		text string
		want []span
	}{
		{"", nil},
		{"просто текст", []span{{spanPlain, "просто текст"}}},
		{"**Шаблоны** (Ctrl+T)", []span{{spanBold, "Шаблоны"}, {spanPlain, " (Ctrl+T)"}}},
		{"окно `req_access` и **доступы**", []span{{spanPlain, "окно "}, {spanCode, "req_access"}, {spanPlain, " и "}, {spanBold, "доступы"}}},
		// A marker without a pair is a usual text.
		{"2 ** 3", []span{{spanPlain, "2 ** 3"}}},
		{"один ` символ", []span{{spanPlain, "один ` символ"}}},
		{"****", []span{{spanPlain, "****"}}},
		{"`**не жирный**`", []span{{spanCode, "**не жирный**"}}},
	}
	for _, c := range cases {
		if got := parseInline(c.text); !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseInline(%q) = %+v, want %+v", c.text, got, c.want)
		}
	}
}

func runeWidth(s string) float32 {
	return float32(utf8.RuneCountInString(s))
}

func TestLayoutSpans(t *testing.T) {
	spans := parseInline("**один** два `три` четыре")

	// Everything fits one line.
	got := layoutSpans(spans, 100, runeWidth)
	want := [][]span{{{spanBold, "один"}, {spanPlain, " два "}, {spanCode, "три"}, {spanPlain, " четыре"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("one line: %+v", got)
	}

	// Words are moved to the next line as a whole, spaces at the end of a line are removed.
	got = layoutSpans(spans, 9, runeWidth)
	want = [][]span{
		{{spanBold, "один"}, {spanPlain, " два"}},
		{{spanCode, "три"}},
		{{spanPlain, "четыре"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wrapped: %+v", got)
	}
}

func TestLayoutSpansKeepsUTF8AndLongWords(t *testing.T) {
	got := layoutSpans([]span{{spanBold, "оченьдлинноеслово"}, {spanPlain, " а"}}, 5, runeWidth)
	want := [][]span{{{spanBold, "оченьдлинноеслово"}}, {{spanPlain, "а"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("long word: %+v", got)
	}
	for _, line := range got {
		for _, s := range line {
			if !utf8.ValidString(s.text) {
				t.Fatalf("broken text: %q", s.text)
			}
		}
	}
}

func TestLayoutSpansLineBreak(t *testing.T) {
	got := layoutSpans([]span{{spanBold, "а\nб"}}, 100, runeWidth)
	want := [][]span{{{spanBold, "а"}}, {{spanBold, "б"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("line break: %+v", got)
	}
}

func TestParseEmptyChangelog(t *testing.T) {
	if !Parse("").IsEmpty() {
		t.Fatal("an empty text gives an empty markdown")
	}
}
