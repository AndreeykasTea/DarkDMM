package dmaccess

import (
	"os"
	"path/filepath"
	"testing"
)

// Paradise: #define JOB_TITLE_RU_REPRESENTATIVE "Представитель \"Нанотрейзен\"".
func TestJobTitleWithEscapedQuotes(t *testing.T) {
	root := t.TempDir()
	write := func(rel, text string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("code/__DEFINES/jobs/job_titles_ru.dm", "#define JOB_TITLE_RU_REPRESENTATIVE \"Представитель \\\"Нанотрейзен\\\"\"\n"+
		"#define JOB_TITLE_RU_CAPTAIN \"Капитан\"\n")
	write("code/game/jobs/job/central.dm", "/datum/job/nanotrasenrep\n\ttitle = JOB_TITLE_REPRESENTATIVE\n\taccess = list(ACCESS_CENT_GENERAL)\n\n"+
		"/datum/job/captain\n\ttitle = JOB_TITLE_CAPTAIN\n\taccess = list(ACCESS_CENT_GENERAL)\n")

	byDefine := map[string]*Access{"ACCESS_CENT_GENERAL": {Define: "ACCESS_CENT_GENERAL", Value: "101"}}
	jobs := loadJobs(root, byDefine)
	names := map[string]bool{}
	for _, job := range jobs {
		names[job.Name] = true
	}
	for _, want := range []string{`Представитель "Нанотрейзен"`, "Капитан"} {
		if !names[want] {
			t.Errorf("no job %q in %v", want, names)
		}
	}
}

func TestUnescapeDM(t *testing.T) {
	for in, want := range map[string]string{
		`Капитан`:           "Капитан",
		`\"Нанотрейзен\"`:   `"Нанотрейзен"`,
		`\[Должность\] Имя`: "[Должность] Имя",
		`back\\slash`:       `back\slash`,
		`trailing\`:         `trailing\`,
	} {
		if got := unescapeDM(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}
