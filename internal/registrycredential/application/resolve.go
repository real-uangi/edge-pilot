package application

import "strings"

func ResolveRegistryHost(imageRepo string) string {
	repo := strings.TrimSpace(imageRepo)
	if repo == "" {
		return "docker.io"
	}
	first := repo
	if idx := strings.Index(first, "/"); idx >= 0 {
		first = first[:idx]
	}
	first = strings.ToLower(strings.TrimSpace(first))
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return first
	}
	return "docker.io"
}
