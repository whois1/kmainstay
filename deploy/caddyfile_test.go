package deploy

import (
	"os"
	"strings"
	"testing"
)

func TestCaddyContentSecurityPolicyAllowsLocalImagePreviews(t *testing.T) {
	configuration, err := os.ReadFile("Caddyfile")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(configuration), "img-src 'self' data: blob:;") {
		t.Fatal("Content-Security-Policy must allow blob image URLs used by selected-image previews")
	}
}
