package s3

import (
	"path"
	"path/filepath"
	"strings"
)

// CacheControlFor returns the Cache-Control header value for a file (given its path relative
// to the artifact root) based on the supplied rules.
//
// When rules is nil (the module emitted no cache_control_rules, e.g. revalidate_html_pages is
// off), it returns "" so the uploader leaves Cache-Control unset — preserving prior behavior.
// Otherwise, paths matching any RevalidateGlobs get RevalidateHeader; everything else gets
// ImmutableHeader.
func CacheControlFor(rules *CacheControlRules, relPath string) string {
	if rules == nil {
		return ""
	}
	for _, g := range rules.RevalidateGlobs {
		if matchGlob(g, relPath) {
			return rules.RevalidateHeader
		}
	}
	return rules.ImmutableHeader
}

// matchGlob matches relPath against glob using stdlib path.Match. It additionally supports the
// "**/<pattern>" shape emitted by the module (path.Match has no "**"): a leading "**/" matches
// any directory depth, so the remainder is matched against the file's basename.
func matchGlob(glob, relPath string) bool {
	p := filepath.ToSlash(relPath)
	if rest, ok := strings.CutPrefix(glob, "**/"); ok {
		matched, _ := path.Match(rest, path.Base(p))
		return matched
	}
	matched, _ := path.Match(glob, p)
	return matched
}
