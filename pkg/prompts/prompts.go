package prompts

import (
	"fmt"
	"os"
	"strings"
)

// Load reads a prompt file and replaces placeholders with provided values.
func Load(name string, replacements map[string]string) (string, error) {
	// Try looking in prompts/ and if that fails, try ../prompts/, ../../prompts/
	paths := []string{
		fmt.Sprintf("prompts/%s.md", name),
		fmt.Sprintf("../prompts/%s.md", name),
		fmt.Sprintf("../../prompts/%s.md", name),
	}

	var data []byte
	var err error
	for _, path := range paths {
		data, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to read prompt file %s: %v", name, err)
	}

	prompt := string(data)
	for key, value := range replacements {
		prompt = strings.ReplaceAll(prompt, fmt.Sprintf("{{%s}}", key), value)
	}

	return prompt, nil
}
