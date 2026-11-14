package image

import (
	"strings"
	"testing"
)

func TestValidTag(t *testing.T) {
	for _, tag := range []string{"latest", "v1.2.3", "_build", "Release-42", strings.Repeat("a", 128)} {
		if !ValidTag(tag) {
			t.Errorf("ValidTag(%q) = false", tag)
		}
	}
	for _, tag := range []string{"", ".version", "-version", "tag:name", "../latest", "a/b", " latest", "标签", strings.Repeat("a", 129)} {
		if ValidTag(tag) {
			t.Errorf("ValidTag(%q) = true", tag)
		}
	}
}
