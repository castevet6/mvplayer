package app

import (
	"fmt"
	"os"
	"path/filepath"
)

// Startup describes the paths the application should browse and, when set,
// play after the playback adapter is added.
type Startup struct {
	BrowseDir string
	PlayFile  string
	Help      bool
}

// ResolveArgs validates command-line arguments and resolves them against cwd.
func ResolveArgs(args []string, cwd string) (Startup, error) {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		return Startup{Help: true}, nil
	}
	if len(args) > 1 {
		return Startup{}, fmt.Errorf("expected at most one path argument, got %d", len(args))
	}
	if len(args) == 1 && len(args[0]) > 1 && args[0][0] == '-' {
		return Startup{}, fmt.Errorf("unknown option %q (use --help for usage)", args[0])
	}
	if cwd == "" {
		return Startup{}, fmt.Errorf("resolve current directory: empty working directory")
	}

	target := cwd
	if len(args) == 1 {
		target = args[0]
		if target == "" {
			return Startup{}, fmt.Errorf("path argument must not be empty")
		}
	}

	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Startup{}, fmt.Errorf("resolve path %q: %w", target, err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return Startup{}, fmt.Errorf("access startup path %q: %w", target, err)
	}
	if info.IsDir() {
		if _, err := os.ReadDir(absolute); err != nil {
			return Startup{}, fmt.Errorf("read startup directory %q: %w", absolute, err)
		}
		return Startup{BrowseDir: absolute}, nil
	}
	if !info.Mode().IsRegular() {
		return Startup{}, fmt.Errorf("startup path %q is neither a regular file nor a directory", target)
	}
	file, err := os.Open(absolute)
	if err != nil {
		return Startup{}, fmt.Errorf("open startup file %q: %w", absolute, err)
	}
	if err := file.Close(); err != nil {
		return Startup{}, fmt.Errorf("close startup file %q: %w", absolute, err)
	}
	return Startup{BrowseDir: filepath.Dir(absolute), PlayFile: absolute}, nil
}
