// Package version defines the version exposed by a running Peck build.
package version

import (
	"errors"
	"regexp"
	"strings"
)

var appVersionPattern = regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)

// Current is the development fallback used by tests and callers that do not
// provide the packaged manifest version.
const Current = "dev"

func Normalize(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "v")
}

func FromManifest(data string) (string, error) {
	match := appVersionPattern.FindStringSubmatch(data)
	if len(match) != 2 || Normalize(match[1]) == "" {
		return "", errors.New("application version is missing from viva.toml")
	}
	return Normalize(match[1]), nil
}
