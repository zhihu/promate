package prometheus

// Copyright (c) 2019 Uber Technologies, Inc.
// [Apache License 2.0](licenses/m3.license.txt)

// Fork from https://github.com/m3db/m3/blob/e098969502ff32abe4d6be536cc7c1cf06885a85/src/query/graphite/graphite/glob_test.go

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobToRegexPattern(t *testing.T) {
	tests := []struct {
		glob    string
		isRegex bool
		regex   string
	}{
		{
			glob:    "barbaz",
			isRegex: false,
			regex:   "barbaz",
		},
		{
			glob:    "barbaz:quxqaz",
			isRegex: false,
			regex:   "barbaz:quxqaz",
		},
		{
			glob:    "foo\\+bar.'baz<1001>'.qux",
			isRegex: true,
			regex:   "foo\\+bar\\.+\\'baz\\<1001\\>\\'\\.+qux",
		},
		{
			glob:    "foo.host.me{1,2,3}.*",
			isRegex: true,
			regex:   "foo\\.+host\\.+me(1|2|3)\\.+[^.]*",
		},
		{
			glob:    "bar.zed.whatever[0-9].*.*.bar",
			isRegex: true,
			regex:   "bar\\.+zed\\.+whatever[0-9]\\.+[^.]*\\.+[^.]*\\.+bar",
		},
		{
			glob:    "foo{0[3-9],1[0-9],20}",
			isRegex: true,
			regex:   "foo(0[3-9]|1[0-9]|20)",
		},
		{
			glob:    "foo{0[3-9],1[0-9],20}:bar",
			isRegex: true,
			regex:   "foo(0[3-9]|1[0-9]|20):bar",
		},
	}

	for _, test := range tests {
		pattern, isRegex, err := globToRegexPattern(test.glob)
		require.NoError(t, err)
		assert.Equal(t, test.isRegex, isRegex)
		assert.Equal(t, test.regex, pattern, "bad pattern for %s", test.glob)
	}
}

func TestGlobToRegexPatternErrors(t *testing.T) {
	tests := []struct {
		glob string
		err  string
	}{
		{"foo.host{1,2", "unbalanced '{' in foo.host{1,2"},
		{"foo.host{1,2]", "invalid ']' at 12, no prior for '[' in foo.host{1,2]"},
		{"foo.,", "invalid ',' outside of matching group at pos 4 in foo.,"},
		{"foo.host{a[0-}", "invalid '}' at 13, no prior for '{' in foo.host{a[0-}"},
	}

	for _, test := range tests {
		_, _, err := globToRegexPattern(test.glob)
		require.Error(t, err)
		assert.Equal(t, test.err, err.Error(), "invalid error for %s", test.glob)
	}
}

func TestCompileGlob(t *testing.T) {
	tests := []struct {
		glob    string
		match   bool
		toMatch []string
	}{
		{"foo.bar.timers.baz??-bar.qux.query.count", true,
			[]string{
				"foo.bar.timers.baz01-bar.qux.query.count",
				"foo.bar.timers.baz24-bar.qux.query.count"}},
		{"foo.bar.timers.baz??-bar.qux.query.count", false,
			[]string{
				"foo.bar.timers.baz-bar.qux.query.count",
				"foo.bar.timers.baz.0-bar.qux.query.count",
				"foo.bar.timers.baz021-bar.qux.query.count",
				"foo.bar.timers.baz991-bar.qux.query.count"}},
		{"foo.host{1,2}.*", true,
			[]string{"foo.host1.zed", "foo.host2.whatever"}},
		{"foo.*.zed.*", true,
			[]string{"foo.bar.zed.eq", "foo.zed.zed.zed"}},
		{"foo.*.zed.*", false,
			[]string{"bar.bar.zed.zed", "foo.bar.zed", "foo.bar.zed.eq.monk"}},
		{"foo.host{1,2}.zed", true,
			[]string{"foo.host1.zed", "foo.host2.zed"}},
		{"foo.host{1,2}.zed", false,
			[]string{"foo.host3.zed", "foo.hostA.zed", "blad.host1.zed", "foo.host1.zed.z"}},
		{"optic{0[3-9],1[0-9],20}", true,
			[]string{"optic03", "optic10", "optic20"}},
		{"optic{0[3-9],1[0-9],20}", false,
			[]string{"optic01", "optic21", "optic201", "optic031"}},
	}

	for _, test := range tests {
		rePattern, _, err := globToRegexPattern(test.glob)
		require.NoError(t, err)
		re := regexp.MustCompile(fmt.Sprintf("^%s$", rePattern))
		for _, s := range test.toMatch {
			matched := re.MatchString(s)
			assert.Equal(t, test.match, matched, "incorrect match between %s and %s", test.glob, s)
		}
	}
}
