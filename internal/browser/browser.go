package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry describes a browser row. Path always identifies the filesystem item;
// RelativePath is populated for recursive media and navigation rows.
type Entry struct {
	Name         string
	Path         string
	RelativePath string
	Dir          bool
	Media        bool
}

// ListRecursive reads media beneath path without descending into directory
// symlinks. Immediate directories, including directory symlinks, are returned
// first as navigation rows. A descendant read failure is reported as a warning
// while other readable paths are kept.
func ListRecursive(ctx context.Context, path string) ([]Entry, string, error) {
	return listRecursiveWithReadDir(ctx, path, os.ReadDir)
}

func listRecursiveWithReadDir(ctx context.Context, path string, readDir func(string) ([]os.DirEntry, error)) ([]Entry, string, error) {
	root, err := filepath.Abs(path)
	if err != nil {
		return nil, "", fmt.Errorf("resolve directory %q: %w", path, err)
	}
	var folders, media []Entry
	var failed string
	skipped := 0
	var walk func(string, bool) error
	walk = func(dir string, isRoot bool) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		items, err := readDir(dir)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			if isRoot {
				return fmt.Errorf("read directory %q: %w", dir, err)
			}
			skipped++
			if failed == "" {
				failed = dir
			}
			return nil
		}
		for _, item := range items {
			if err := ctx.Err(); err != nil {
				return err
			}
			name := item.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			full := filepath.Join(dir, name)
			rel, err := filepath.Rel(root, full)
			if err != nil {
				return err
			}
			isDir := item.IsDir()
			isSymlinkDir := !isDir && isDirectorySymlink(item, full)
			if isSymlinkDir {
				if isRoot {
					folders = append(folders, Entry{Name: name, Path: full, RelativePath: name, Dir: true})
				}
				continue
			}
			if isDir {
				if isRoot {
					folders = append(folders, Entry{Name: name, Path: full, RelativePath: name, Dir: true})
				}
				if err := walk(full, false); err != nil {
					return err
				}
				continue
			}
			if _, ok := mediaExtensions[strings.ToLower(filepath.Ext(name))]; !ok {
				continue
			}
			media = append(media, Entry{Name: rel, Path: full, RelativePath: rel, Media: true})
		}
		return ctx.Err()
	}
	if err := walk(root, true); err != nil {
		return nil, "", err
	}
	sortEntries(folders, true)
	sortEntries(media, true)
	entries := append(folders, media...)
	warning := ""
	if skipped > 0 {
		warning = fmt.Sprintf("could not read %q; skipped %d subtree(s)", failed, skipped)
	}
	return entries, warning, nil
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
		isDir := item.IsDir() || isDirectorySymlink(item, filepath.Join(path, name))
		media := false
		if !isDir {
			_, media = mediaExtensions[strings.ToLower(filepath.Ext(name))]
			if !media {
				continue
			}
		}
		entries = append(entries, Entry{Name: name, Path: filepath.Join(path, name), Dir: isDir, Media: media})
	}
	sortEntries(entries, false)
	return entries, nil
}

func isDirectorySymlink(item os.DirEntry, path string) bool {
	if item.Type()&os.ModeSymlink == 0 {
		return false
	}
	target, err := os.Stat(path)
	return err == nil && target.IsDir()
}

func sortEntries(entries []Entry, byRelative bool) {
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i].Name, entries[j].Name
		if byRelative {
			a, b = entries[i].RelativePath, entries[j].RelativePath
		} else {
			return strings.ToLower(a) < strings.ToLower(b)
		}
		if strings.ToLower(a) != strings.ToLower(b) {
			return strings.ToLower(a) < strings.ToLower(b)
		}
		return a < b
	})
}

// Filter returns entries whose names contain query, case-insensitively.
func Filter(entries []Entry, query string) []Entry {
	if query == "" {
		return entries
	}
	needle := strings.ToLower(query)
	filtered := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		match := entry.Name
		if entry.RelativePath != "" {
			match = entry.RelativePath
		}
		if strings.Contains(strings.ToLower(match), needle) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
