package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/ai-automation-tools/ai-security-command-center/src/internal/claudecode"
	"github.com/ai-automation-tools/ai-security-command-center/src/internal/pdp"
	"github.com/ai-automation-tools/ai-security-command-center/src/internal/version"
)

const maxConfigBytes = 16 << 20 // a real .claude.json is a few megabytes at most

// discover walks root for the agent config files ascc knows and writes what the registry doesn't
// list as an inventory document. It reads only. Exit 0 means everything under root was read, 1 that
// the document is incomplete (each gap is named on stderr), 2 that ascc couldn't run.
func discover(registryPath, policies, auditLog, root, out string, stdout, stderr io.Writer) int {
	data, err := os.ReadFile(registryPath)
	if err != nil {
		return fail(stderr, fmt.Errorf("registry: %v", err))
	}
	reg, err := pdp.ParseRegistry(data)
	if err != nil {
		return fail(stderr, err)
	}
	// Resolve a linked root first: the walk doesn't follow links, and would find nothing behind one.
	root, err = filepath.EvalSymlinks(root)
	if st, statErr := os.Stat(root); err != nil || statErr != nil || !st.IsDir() {
		return fail(stderr, errors.New("discover: --root isn't a folder ascc can read"))
	}

	files := map[string][]byte{}
	var gaps []string
	gap := func(rel, why string) {
		gaps = append(gaps, fmt.Sprintf("ascc: discover: %s %s, so what it holds isn't in this report", rel, why))
	}
	// ponytail: the whole tree is walked, bar .git and node_modules. Add a depth limit or a skip list
	// if a home folder ever makes this slow. A linked folder isn't followed (links can loop); it is
	// reported as a gap instead.
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			rel = d.Name()
		}
		rel = filepath.ToSlash(rel)
		switch {
		case err != nil:
			gap(rel, "couldn't be read")
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
		case d.IsDir():
			if rel != "." && (d.Name() == ".git" || d.Name() == "node_modules") {
				return fs.SkipDir
			}
		default:
			known := claudecode.IsLocation(rel)
			if d.Type().IsRegular() && !known {
				return nil // an ordinary file ascc has no interest in
			}
			// Stat follows a link or junction, so this sees what a session opening the path would get.
			st, statErr := os.Stat(p)
			switch {
			case statErr == nil && st.IsDir():
				gap(rel, "is a link to a folder, which ascc doesn't follow")
			case !known:
			case statErr != nil || !st.Mode().IsRegular():
				gap(rel, "couldn't be read")
			case st.Size() > maxConfigBytes:
				gap(rel, "is too large to be a config file")
			default:
				if data, err := os.ReadFile(p); err != nil {
					gap(rel, "couldn't be read")
				} else {
					files[rel] = data
				}
			}
		}
		return nil
	})

	doc, unreadable := claudecode.Discover(files, reg, version.Version, time.Now())
	for _, rel := range unreadable {
		gap(rel, "isn't a JSON object")
	}
	if err := writeDoc(out, stdout, doc, registryPath, policies, auditLog); err != nil {
		return fail(stderr, err)
	}
	for _, line := range gaps {
		fmt.Fprintln(stderr, line)
	}
	if len(gaps) > 0 {
		return 1
	}
	return 0
}
