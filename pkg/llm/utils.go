package llm

import "strings"

func extractJSON(input string) string {
	start := strings.Index(input, "{")
	end := strings.LastIndex(input, "}")
	if start != -1 && end != -1 && end >= start {
		return input[start : end+1]
	}
	return input
}
