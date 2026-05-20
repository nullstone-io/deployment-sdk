package s3

import "testing"

func TestCacheControlFor(t *testing.T) {
	rules := &CacheControlRules{
		RevalidateGlobs:  []string{"**/*.html"},
		RevalidateHeader: "no-cache",
		ImmutableHeader:  "public, max-age=31536000, immutable",
	}

	tests := []struct {
		name    string
		rules   *CacheControlRules
		relPath string
		want    string
	}{
		{name: "nil rules leaves header unset", rules: nil, relPath: "index.html", want: ""},
		{name: "root html revalidates", rules: rules, relPath: "index.html", want: "no-cache"},
		{name: "nested html revalidates", rules: rules, relPath: "nested/app.html", want: "no-cache"},
		{name: "windows separator nested html", rules: rules, relPath: `nested\app.html`, want: "no-cache"},
		{name: "hashed js is immutable", rules: rules, relPath: "main.abc123.js", want: "public, max-age=31536000, immutable"},
		{name: "nested css is immutable", rules: rules, relPath: "assets/x.css", want: "public, max-age=31536000, immutable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CacheControlFor(tt.rules, tt.relPath); got != tt.want {
				t.Errorf("CacheControlFor(%q) = %q, want %q", tt.relPath, got, tt.want)
			}
		})
	}
}
