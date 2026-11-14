package image

import "regexp"

var tagPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)

func ValidTag(tag string) bool {
	return tagPattern.MatchString(tag)
}
