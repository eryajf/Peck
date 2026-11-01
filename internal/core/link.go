package core

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// LinkRequest contains hints, never a path or permission to modify an app.
type LinkRequest struct{ Project, BundleID, Release, Homepage string }

const LinkScheme = "peck"

var bundleIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*$`)
var domainPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$`)

func ParseLink(raw string) (LinkRequest, error) {
	var r LinkRequest
	bad := errors.New("链接格式不支持，请从项目页面重新打开 Peck 链接")
	if len(raw) > 4096 || !utf8.ValidString(raw) {
		return r, bad
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != LinkScheme || u.Host != "prepare" || u.User != nil || u.Path != "" || u.Fragment != "" || u.Opaque != "" {
		return r, bad
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return r, bad
	}
	for k, vals := range q {
		if len(vals) != 1 || vals[0] == "" || !safeText(vals[0]) {
			return r, bad
		}
		switch k {
		case "v", "project", "bundle_id", "release", "homepage":
		default:
			return r, bad
		}
	}
	if q.Get("v") != "1" || strings.TrimSpace(q.Get("project")) == "" || utf8.RuneCountInString(q.Get("project")) > 128 {
		return r, bad
	}
	r = LinkRequest{q.Get("project"), q.Get("bundle_id"), q.Get("release"), q.Get("homepage")}
	if r.BundleID != "" && (len(r.BundleID) > 255 || !bundleIDPattern.MatchString(r.BundleID)) || utf8.RuneCountInString(r.Release) > 64 {
		return LinkRequest{}, bad
	}
	if r.Homepage != "" {
		h, err := url.Parse(r.Homepage)
		if err != nil || h.Scheme != "https" || h.Hostname() == "" || h.User != nil || utf8.RuneCountInString(r.Homepage) > 2048 {
			return LinkRequest{}, bad
		}
		host := strings.ToLower(strings.TrimSuffix(h.Hostname(), "."))
		if net.ParseIP(host) != nil || !domainPattern.MatchString(host) || h.Port() != "" && h.Port() != "443" {
			return LinkRequest{}, bad
		}
		for _, suffix := range []string{"localhost", ".localhost", ".local", ".lan", ".internal", ".home", ".home.arpa"} {
			if host == suffix || strings.HasSuffix(host, suffix) {
				return LinkRequest{}, bad
			}
		}
		if _, err := url.QueryUnescape(h.RawQuery); err != nil {
			return LinkRequest{}, bad
		}
	}
	return r, nil
}

func safeText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}
