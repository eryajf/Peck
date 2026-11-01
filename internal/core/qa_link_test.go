package core

import (
	"net/url"
	"strings"
	"testing"
)

func TestQADeepLinkPublicContract(t *testing.T) {
	valid := []string{
		"peck://prepare?v=1&project=Example%20App&bundle_id=org.example.app&release=1.2.3&homepage=https%3A%2F%2Fexample.invalid%2Fapp",
		"peck://prepare?v=1&project=" + url.QueryEscape(strings.Repeat("啄", 128)),
		"peck://prepare?v=1&project=App&homepage=" + url.QueryEscape("https://example.invalid/app?a=1&b=2"),
	}
	for _, raw := range valid {
		if _, err := ParseLink(raw); err != nil {
			t.Errorf("valid public contract rejected: %q: %v", raw, err)
		}
	}
	r, err := ParseLink(valid[0])
	if err == nil && (r.Project != "Example App" || r.BundleID != "org.example.app" || r.Release != "1.2.3" || r.Homepage != "https://example.invalid/app") {
		t.Fatalf("decoded hints differ from published example: %+v", r)
	}
}

func TestQADeepLinkRejectsAuthorityAndMalformedHints(t *testing.T) {
	base := "peck://prepare?v=1&project=App"
	invalid := []string{
		base + "&path=%2FApplications%2FApp.app",
		base + "&file=file%3A%2F%2F%2FApplications%2FApp.app",
		base + "&command=touch", base + "&args=anything", base + "&auto_sign=true",
		base + "&sudo=true", base + "&callback=https%3A%2F%2Fexample.invalid",
		base + "&project=Other", base + "&v=1", base + "&bundle_id=org..app",
		base + "&bundle_id=org.app.", base + "&bundle_id=.org.app",
		"peck://prepare?v=2&project=App", "peck://prepare?v=1&project=",
		"peck://prepare?v=1&project=%00App", "peck://prepare?v=1&project=%FF",
		"peck://prepare?v=1&project=%ZZ", "peck://prepare?v=1&project=App%0AName",
		"peck://prepare/anything?v=1&project=App", "peck://someone@prepare?v=1&project=App",
		"peck://prepare:12?v=1&project=App", base + "#fragment",
		"https://prepare?v=1&project=App", "peck://sign?v=1&project=App",
		"peckmac://prepare?v=1&project=App",
		"peck://prepare?v=1&project=" + url.QueryEscape(strings.Repeat("x", 129)),
		base + "&release=" + strings.Repeat("x", 65),
		base + strings.Repeat("x", 4096),
	}
	for _, homepage := range []string{
		"http://example.invalid/app", "file:///Applications/App.app", "javascript:alert(1)",
		"https://user:secret@example.invalid/app", "https://127.0.0.1/app", "https://[::1]/app",
		"https://192.168.1.1/app", "https://localhost/app", "https://host.local/app",
		"https://example.invalid:8443/app",
	} {
		invalid = append(invalid, base+"&homepage="+url.QueryEscape(homepage))
	}
	for _, raw := range invalid {
		t.Run(raw, func(t *testing.T) {
			if _, err := ParseLink(raw); err == nil {
				t.Fatal("untrusted or malformed link accepted")
			}
		})
	}
}
