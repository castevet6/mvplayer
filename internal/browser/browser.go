package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry describes a visible child of a directory.
type Entry struct {
	Name  string
	Path  string
	Dir   bool
	Media bool
}

var mediaExtensions = map[string]struct{}{
	".mp4": {}, ".m4v": {}, ".mkv": {}, ".avi": {}, ".mov": {}, ".webm": {},
	".mpg": {}, ".mpeg": {}, ".wmv": {}, ".flv": {}, ".3gp": {},
	".mp3": {}, ".flac": {}, ".ogg": {}, ".opus": {}, ".m4a": {}, ".wav": {},
	".aac": {}, ".wma": {}, ".aiff": {}, ".ape": {},
}

// List reads one directory, omitting hidden and unsupported files.
func List(path string) ([]Entry, error) {
	items, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", path, err)
	}
	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		name := item.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		isDir := item.IsDir()
		media := false
		if !isDir {
			_, media = mediaExtensions[strings.ToLower(filepath.Ext(name))]
			if !media {
				continue
			}
		}
		entries = append(entries, Entry{Name: name, Path: filepath.Join(path, name), Dir: isDir, Media: media})
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

// Filter returns entries whose names contain query, case-insensitively.
func Filter(entries []Entry, query string) []Entry {
	if query == "" {
		return entries
	}
	needle := strings.ToLower(query)
	filtered := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Name), needle) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
