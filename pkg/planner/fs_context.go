package planner

import (
	"bytes"
	"os/exec"
	"strings"
)

// GetFileSystemTree returns a newline-separated list of files in the given directory.
// It uses `git ls-files` to strictly respect .gitignore patterns.
func GetFileSystemTree(dir string) string {
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		return strings.TrimSpace(out.String())
	}
	return ""
}
