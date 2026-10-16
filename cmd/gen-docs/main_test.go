package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/rtorcato/homelab-nut/internal/cli"
	"github.com/spf13/cobra"
)

// Every command keeps cobra's flat slug whatever folder its page lands in,
// and every page has exactly one H1.
func TestGenerateKeepsSlugsAndOneTitle(t *testing.T) {
	root := cli.NewRootForDocs(cli.BuildInfo{Version: "vX.Y.Z"})
	dir := t.TempDir()
	if err := generate(root, dir); err != nil {
		t.Fatal(err)
	}

	var want []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		want = append(want, "/cli/"+slug(c))
		for _, sc := range children(c) {
			walk(sc)
		}
	}
	walk(root)

	slugRe := regexp.MustCompile(`(?m)^slug: (.+)$`)
	h1Re := regexp.MustCompile("(?m)^# `[^`]+`$")
	var got []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		m := slugRe.FindSubmatch(b)
		if m == nil {
			t.Errorf("%s: no slug", p)
			return nil
		}
		got = append(got, string(m[1]))
		if n := len(h1Re.FindAll(b, -1)); n != 1 {
			t.Errorf("%s: %d code H1s, want 1", p, n)
		}
		if strings.Contains(string(b), "## "+root.Name()) || strings.Contains(string(b), "SEE ALSO") {
			t.Errorf("%s: still has cobra's own heading", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("slugs:\n got %v\nwant %v", got, want)
	}
}
