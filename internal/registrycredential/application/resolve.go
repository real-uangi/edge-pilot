package application

import "strings"

func ResolveRegistryHost(imageRepo string) string {
	repo := strings.TrimSpace(imageRepo)
	if repo == "" {
		return "docker.io"
	}
	first, _, hasSlash := strings.Cut(repo, "/")
	if !hasSlash {
		return "docker.io"
	}
	first = strings.ToLower(strings.TrimSpace(first))
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return first
	}
	return "docker.io"
}
