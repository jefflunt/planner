package planner

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GetFileSystemTree returns a newline-separated list of files in the given directory.
// It prioritizes using `git ls-files` to respect .gitignore, and falls back to a
// standard directory walk if git is unavailable or the directory isn't a repository.
func GetFileSystemTree(dir string) string {
	// Try git first to respect .gitignore
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		return strings.TrimSpace(out.String())
	}

	// Fallback to basic walk
	var files []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Basic ignores for the fallback
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "bin" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			files = append(files, rel)
		}
		return nil
	})
	return strings.Join(files, "\n")
}
