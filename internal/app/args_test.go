package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveArgs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "media space-雪")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "été vidéo.mp4")
	if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "missing")

	tests := []struct {
		name    string
		args    []string
		cwd     string
		want    Startup
		wantErr string
	}{
		{name: "default current directory", cwd: root, want: Startup{BrowseDir: root}},
		{name: "directory argument", args: []string{dir}, cwd: root, want: Startup{BrowseDir: dir}},
		{name: "file argument with spaces and unicode", args: []string{file}, cwd: root, want: Startup{BrowseDir: dir, PlayFile: file}},
		{name: "relative file argument", args: []string{filepath.Join("media space-雪", "été vidéo.mp4")}, cwd: root, want: Startup{BrowseDir: dir, PlayFile: file}},
		{name: "long help", args: []string{"--help"}, cwd: root, want: Startup{Help: true}},
		{name: "short help", args: []string{"-h"}, cwd: root, want: Startup{Help: true}},
		{name: "missing path", args: []string{missing}, cwd: root, wantErr: "access startup path"},
		{name: "unknown option", args: []string{"--bogus"}, cwd: root, wantErr: "unknown option"},
		{name: "excess arguments", args: []string{"one", "two"}, cwd: root, wantErr: "at most one path"},
		{name: "empty path", args: []string{""}, cwd: root, wantErr: "must not be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveArgs(tt.args, tt.cwd)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ResolveArgs() error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveArgs() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResolveArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
