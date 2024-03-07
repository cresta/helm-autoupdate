package helm

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"
)

// Autochange is a Helm plugin that automatically changes the chart version.
// Example:
// charts:
//   - identity: datadog
//     chart:
//     repository: s3://cresta-helm-charts-v2/monorepo/cresta-datadog
//     name: cresta-datadog
//     version: "*"
type Autochange struct {
	Charts        []AutoUpdateCharts `json:"charts"`
	FilenameRegex []string           `json:"filename_regex,omitempty"`
	ParsedRegex   []*regexp.Regexp   `json:"-"`
}

func (a *Autochange) findUpdateChartForUpdate(u *Update) *AutoUpdateChart {
	for _, chart := range a.Charts {
		if chart.Identity == u.Parse.Identity {
			return &chart.Chart
		}
	}
	return nil
}

func CheckForUpdate(il IndexLoader, desc *AutoUpdateChart, request *Update) (*Update, error) {
	indexFile, err := il.LoadIndexFile(desc.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to load index file: %w", err)
	}
	cv, err := indexFile.Get(desc.Name, desc.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get version for chart %s: %w", desc.Name, err)
	}
	if strings.TrimSpace(cv.Version) == strings.TrimSpace(request.Parse.CurrentVersion) {
		return nil, nil
	}
	ret := *request
	tmpParse := *request.Parse
	ret.Parse = &tmpParse
	ret.Parse.CurrentVersion = cv.Version
	return &ret, nil
}

type AutoUpdateCharts struct {
	Identity string          `json:"identity"`
	Chart    AutoUpdateChart `json:"chart"`
}

type AutoUpdateChart struct {
	Repository string `json:"repository"`
	Name       string `json:"name"`
	Version    string `json:"version"`
}

type ChangeFinder interface {
	FindRequestedChanges() ([]*ParsedFile, error)
}

type DirectorySearchForChanges struct {
	Dir string
}

func WriteChangesToFilesystem(files []*ParsedFile) error {
	for _, file := range files {
		if err := os.WriteFile(file.OriginalFilename, file.Bytes(), file.OriginalPermissions); err != nil {
			return fmt.Errorf("failed to write %s: %w", file.OriginalFilename, err)
		}
	}
	return nil
}

func ApplyUpdatesToFiles(il IndexLoader, config *Autochange, files []*ParsedFile) ([]*ParsedFile, error) {
	modifiedFiles := make([]*ParsedFile, 0)
	for _, file := range files {
		hasModification := false
		for _, update := range file.RequestedUpdates {
			update := update
			uc := config.findUpdateChartForUpdate(&update)
			if uc == nil {
				continue
			}
			newChange, err := CheckForUpdate(il, uc, &update)
			if err != nil {
				return nil, fmt.Errorf("failed to check for update: %w", err)
			}
			if newChange == nil {
				continue
			}
			file.ApplyUpdate(newChange)
			hasModification = true
		}
		if hasModification {
			modifiedFiles = append(modifiedFiles, file)
		}
	}
	return modifiedFiles, nil
}

func PathToLoad(pathRegex []*regexp.Regexp, path string) bool {
	if len(pathRegex) == 0 {
		return true
	}
	for _, r := range pathRegex {
		if r.MatchString(path) {
			return true
		}
	}
	return false
}

func (r *DirectorySearchForChanges) FindRequestedChanges(regexPathFilters []*regexp.Regexp) ([]*ParsedFile, error) {
	var ret []*ParsedFile
	if err := filepath.WalkDir(r.Dir, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		stat, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("unable to stat path %s: %w", path, err)
		}
		if stat.IsDir() {
			return nil
		}
		if !PathToLoad(regexPathFilters, path) {
			return nil
		}
		parsedFile, err := ParseFile(path)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}
		if len(parsedFile.RequestedUpdates) > 0 {
			ret = append(ret, parsedFile)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to walk %s: %w", r.Dir, err)
	}
	return ret, nil
}

func Load(data []byte) (*Autochange, error) {
	var autochange Autochange
	if err := yaml.Unmarshal(data, &autochange); err != nil {
		return nil, fmt.Errorf("failed to unmarshal autochange data: %w", err)
	}
	for _, rgx := range autochange.FilenameRegex {
		r, err := regexp.Compile(rgx)
		if err != nil {
			return nil, fmt.Errorf("failed to compile regex %s: %w", rgx, err)
		}
		autochange.ParsedRegex = append(autochange.ParsedRegex, r)
	}
	return &autochange, nil
}

func LoadFile(path string) (*Autochange, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	return Load(data)
}

type Update struct {
	LineNumber int
	Parse      *LineParse
}

type ParsedFile struct {
	OriginalFilename    string
	OriginalContent     string
	OriginalPermissions fs.FileMode
	Lines               []string
	RequestedUpdates    []Update
}

func (p *ParsedFile) Bytes() []byte {
	var s strings.Builder
	for idx, line := range p.Lines {
		if idx != 0 {
			s.WriteString("\n")
		}
		s.WriteString(line)
	}
	return []byte(s.String())
}

func (p *ParsedFile) ApplyUpdate(update *Update) {
	p.Lines[update.LineNumber] = update.Parse.String()
}

func ParseFile(name string) (*ParsedFile, error) {
	content, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", name, err)
	}
	stat, err := os.Stat(name)
	if err != nil {
		return nil, fmt.Errorf("failed to stat %s: %w", name, err)
	}
	pf := ParseContent(string(content))
	pf.OriginalFilename = name
	pf.OriginalPermissions = stat.Mode()
	return &pf, nil
}

func ParseContent(s string) ParsedFile {
	lines := strings.Split(s, "\n")
	ret := ParsedFile{
		OriginalContent: s,
		Lines:           lines,
	}
	for idx, line := range lines {
		parsed := ParseLine(line)
		if parsed == nil {
			continue
		}
		ret.RequestedUpdates = append(ret.RequestedUpdates, Update{
			LineNumber: idx,
			Parse:      parsed,
		})
	}
	return ret
}

var rgx = regexp.MustCompile(`^([^:]*):([^#]*)# helm:autoupdate:([a-zA-Z0-9-]+)(.*)$`)

type LineParse struct {
	Prefix         string
	CurrentVersion string
	Identity       string
	Suffix         string
}

func (l *LineParse) String() string {
	return fmt.Sprintf("%s: %s # helm:autoupdate:%s%s", l.Prefix, l.CurrentVersion, l.Identity, l.Suffix)
}

func ParseLine(line string) *LineParse {
	matches := rgx.FindStringSubmatch(line)
	if matches == nil {
		return nil
	}
	return &LineParse{
		Prefix:         matches[1],
		CurrentVersion: strings.TrimSpace(matches[2]),
		Identity:       matches[3],
		Suffix:         matches[4],
	}
}
