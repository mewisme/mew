package runtime

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFileURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantFile string // file:// URL form
	}{
		{
			name:     "posix absolute path",
			input:    "/home/user/project/file.ts",
			wantFile: "file:///home/user/project/file.ts",
		},
		{
			name:     "posix path with spaces",
			input:    "/home/user/my project/file.ts",
			wantFile: "file:///home/user/my%20project/file.ts",
		},
		{
			name:     "posix path with unicode",
			input:    "/home/user/ café/文件.ts",
			wantFile: "file:///home/user/%20caf%C3%A9/%E6%96%87%E4%BB%B6.ts",
		},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests, []struct {
			name     string
			input    string
			wantFile string // file:// URL form
		}{
			{
				name:     "windows drive letter",
				input:    `C:\Users\example\app.ts`,
				wantFile: "file:///C:/Users/example/app.ts",
			},
			{
				name:     "windows drive letter with spaces",
				input:    `C:\Users\example\my project\app.ts`,
				wantFile: "file:///C:/Users/example/my%20project/app.ts",
			},
			{
				name:     "windows lower-case drive",
				input:    `d:\data\lib.ts`,
				wantFile: "file:///D:/data/lib.ts",
			},
		}...)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileURL(tt.input)

			// Must start with file://
			if !strings.HasPrefix(got, "file://") {
				t.Errorf("fileURL(%q) = %q, want file:// prefix", tt.input, got)
			}

			// Must match expected canonical form.
			if got != tt.wantFile {
				t.Errorf("fileURL(%q) = %q, want %q", tt.input, got, tt.wantFile)
			}

			// Defect regression: must NOT produce file://C:/ (missing third slash).
			if runtime.GOOS == "windows" {
				if strings.HasPrefix(got, "file://") && !strings.HasPrefix(got, "file:///") {
					t.Errorf("fileURL(%q) = %q, missing third slash — would be interpreted as host C:", tt.input, got)
				}
			}
		})
	}
}

func TestFileURLRoundTripGoPath(t *testing.T) {
	// fileURL must always emit a canonical absolute-path file URL.
	// The result must start with file:/// (three slashes) on all platforms.
	tests := []string{
		"/a/b/c.ts",
		string(filepath.Separator) + "a" + string(filepath.Separator) + "b.ts",
	}
	if runtime.GOOS == "windows" {
		tests = append(tests, `C:\Users\test\app.ts`, `D:\data\file.mjs`)
	}

	for _, p := range tests {
		got := fileURL(p)
		if !strings.HasPrefix(got, "file:///") {
			t.Errorf("fileURL(%q) = %q, must start with file:/// (three slashes)", p, got)
		}
	}
}

func TestFileURLNoDoubleSlashOnPosix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test is for POSIX drive-letter-free paths")
	}
	got := fileURL("/home/user/project/file.ts")
	// Must be file:///home/... not file:////home/...
	if strings.Contains(got, "file:////") {
		t.Errorf("fileURL produced double-slash path: %q", got)
	}
}
