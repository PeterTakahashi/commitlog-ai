package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// CacheMeta stores metadata for parse and link cache validation.
type CacheMeta struct {
	ParserVersion string      `json:"parser_version"`
	Parse         *ParseCache `json:"parse,omitempty"`
	Link          *LinkCache  `json:"link,omitempty"`
}

// ParseCache tracks input/output state for the parse command.
type ParseCache struct {
	Sources []FileInfo `json:"sources"`
	Output  FileInfo   `json:"output"`
}

// LinkCache tracks input/output state for the link command.
type LinkCache struct {
	Sessions FileInfo `json:"sessions"`
	GitHead  string   `json:"git_head"`
	Output   FileInfo `json:"output"`
}

// FileInfo stores file identity for change detection.
type FileInfo struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
}

// Cache manages parse/link cache metadata.
type Cache struct {
	path string
	meta CacheMeta
}

// Load reads cache metadata from .commitlog-ai/cache.json.
// Returns a usable Cache even if the file doesn't exist.
func Load(projectDir string) *Cache {
	c := &Cache{
		path: filepath.Join(projectDir, ".commitlog-ai", "cache.json"),
	}
	data, err := os.ReadFile(c.path)
	if err != nil {
		return c
	}
	json.Unmarshal(data, &c.meta)
	return c
}

// Save writes cache metadata to disk.
func (c *Cache) Save() error {
	data, err := json.MarshalIndent(c.meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0644)
}

// IsParseValid checks if the parse cache is still valid.
func (c *Cache) IsParseValid(parserVersion string, sourceFiles []string) bool {
	if c.meta.ParserVersion != parserVersion {
		return false
	}
	if c.meta.Parse == nil {
		return false
	}

	// Check output file still exists and matches
	if !fileMatchesInfo(c.meta.Parse.Output) {
		return false
	}

	// Check source files match exactly
	currentInfos := statFiles(sourceFiles)
	if len(currentInfos) != len(c.meta.Parse.Sources) {
		return false
	}

	cached := make(map[string]FileInfo)
	for _, fi := range c.meta.Parse.Sources {
		cached[fi.Path] = fi
	}
	for _, fi := range currentInfos {
		prev, ok := cached[fi.Path]
		if !ok || prev.Size != fi.Size || prev.ModTime != fi.ModTime {
			return false
		}
	}

	return true
}

// IsLinkValid checks if the link cache is still valid.
func (c *Cache) IsLinkValid(parserVersion string, sessionsPath string, gitHead string) bool {
	if c.meta.ParserVersion != parserVersion {
		return false
	}
	if c.meta.Link == nil {
		return false
	}

	// Check git HEAD matches
	if c.meta.Link.GitHead != gitHead {
		return false
	}

	// Check sessions.json matches
	si := statFile(sessionsPath)
	if si == nil || si.Size != c.meta.Link.Sessions.Size || si.ModTime != c.meta.Link.Sessions.ModTime {
		return false
	}

	// Check output file still exists and matches
	if !fileMatchesInfo(c.meta.Link.Output) {
		return false
	}

	return true
}

// UpdateParse saves parse cache metadata after a successful parse.
func (c *Cache) UpdateParse(parserVersion string, sourceFiles []string, outputPath string) {
	c.meta.ParserVersion = parserVersion
	c.meta.Parse = &ParseCache{
		Sources: statFiles(sourceFiles),
		Output:  *statFile(outputPath),
	}
	// Invalidate link cache since sessions changed
	c.meta.Link = nil
}

// UpdateLink saves link cache metadata after a successful link.
func (c *Cache) UpdateLink(parserVersion string, sessionsPath string, gitHead string, outputPath string) {
	c.meta.ParserVersion = parserVersion
	c.meta.Link = &LinkCache{
		Sessions: *statFile(sessionsPath),
		GitHead:  gitHead,
		Output:   *statFile(outputPath),
	}
}

func statFile(path string) *FileInfo {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	return &FileInfo{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime().UnixNano(),
	}
}

func statFiles(paths []string) []FileInfo {
	var infos []FileInfo
	for _, p := range paths {
		if fi := statFile(p); fi != nil {
			infos = append(infos, *fi)
		}
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Path < infos[j].Path
	})
	return infos
}

func fileMatchesInfo(fi FileInfo) bool {
	current := statFile(fi.Path)
	if current == nil {
		return false
	}
	return current.Size == fi.Size && current.ModTime == fi.ModTime
}
