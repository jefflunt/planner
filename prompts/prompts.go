package prompts

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed *.md
var FS embed.FS

// Load reads a prompt file from the embedded filesystem and replaces placeholders with provided values.
func Load(name string, replacements map[string]string) (string, error) {
	data, err := FS.ReadFile(fmt.Sprintf("%s.md", name))
	if err != nil {
		return "", fmt.Errorf("failed to read embedded prompt file %s: %v", name, err)
	}

	prompt := string(data)
	for key, value := range replacements {
		prompt = strings.ReplaceAll(prompt, fmt.Sprintf("{{%s}}", key), value)
	}

	return prompt, nil
}
