package dmaccess

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Job is a job of the code base with its accesses (used as presets).
type Job struct {
	Name     string
	Accesses []string // Values as they are written on the map.
}

var (
	reJobBlock  = regexp.MustCompile(`(?m)^/datum/job/[\w/]+\s*$`)
	reJobTitle  = regexp.MustCompile(`(?m)^\s+title\s*=\s*(.+?)\s*$`)
	reJobAccess = regexp.MustCompile(`(?m)^\s+access\s*=\s*list\(([^)]*)\)`)
	// Titles could have escaped quotes: "Представитель \"Нанотрейзен\"".
	reTitleDef   = regexp.MustCompile(`(?m)^\s*#define\s+(JOB_TITLE_\w+)\s+"((?:[^"\\\n]|\\.)+)"`)
	reAccessName = regexp.MustCompile(`ACCESS_[A-Za-z0-9_]+`)
)

// loadJobs reads jobs from files which have "job" in their path. Russian titles are preferred.
func loadJobs(rootDir string, byDefine map[string]*Access) []Job {
	var jobSources []string
	titles := make(map[string]string)
	unusedMaps := unusedMapDirs(rootDir)

	_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != rootDir && (strings.HasPrefix(d.Name(), ".") || skipDirs[d.Name()] || unusedMaps[path]) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".dm") || !strings.Contains(strings.ToLower(path), "job") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		src := string(data)
		for _, m := range reTitleDef.FindAllStringSubmatch(src, -1) {
			if _, ok := titles[m[1]]; !ok {
				titles[m[1]] = unescapeDM(m[2])
			}
		}
		if strings.Contains(src, "/datum/job/") {
			jobSources = append(jobSources, src)
		}
		return nil
	})

	unique := make(map[string]bool)
	var jobs []Job
	for _, src := range jobSources {
		starts := reJobBlock.FindAllStringIndex(src, -1)
		for i, s := range starts {
			end := len(src)
			if i+1 < len(starts) {
				end = starts[i+1][0]
			}
			block := src[s[0]:end]

			tm := reJobTitle.FindStringSubmatch(block)
			am := reJobAccess.FindStringSubmatch(block)
			if tm == nil || am == nil {
				continue
			}
			title := strings.Trim(tm[1], `"`)
			// JOB_TITLE_ENGINEER -> JOB_TITLE_RU_ENGINEER (Paradise has russian titles as separate defines).
			if ru, ok := titles["JOB_TITLE_RU_"+strings.TrimPrefix(title, "JOB_TITLE_")]; ok {
				title = ru
			} else if en, ok := titles[title]; ok {
				title = en
			}
			if unique[title] {
				continue
			}

			var accesses []string
			for _, ident := range reAccessName.FindAllString(am[1], -1) {
				if a, ok := byDefine[ident]; ok {
					accesses = append(accesses, a.Value)
				}
			}
			if len(accesses) == 0 {
				continue
			}
			unique[title] = true
			jobs = append(jobs, Job{Name: title, Accesses: accesses})
		}
	}

	sort.Slice(jobs, func(i, j int) bool { return jobs[i].Name < jobs[j].Name })
	return jobs
}

// unescapeDM removes escapes of a DM string: \" -> ", \\ -> \.
func unescapeDM(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
