// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package reposelector_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eshu-hq/eshu-cli/internal/cli/reposelector"
)

type fakeGetter struct {
	path string
	resp reposelector.ListResponse
	err  error
}

func (f *fakeGetter) Get(path string, result any) error {
	f.path = path
	if f.err != nil {
		return f.err
	}
	if r, ok := result.(*reposelector.ListResponse); ok {
		*r = f.resp
	}
	return nil
}

func TestMatchesExactFields(t *testing.T) {
	repo := reposelector.Entry{
		ID:        "repo-1",
		Name:      "web",
		Path:      "/srv/web",
		LocalPath: "/home/op/web",
		RepoSlug:  "org/web",
	}
	for _, selector := range []string{"repo-1", "web", "/srv/web", "/home/op/web", "org/web"} {
		if !reposelector.Matches(repo, selector) {
			t.Fatalf("Matches(%v, %q) = false, want true", repo, selector)
		}
	}
	if reposelector.Matches(repo, "repo-2") {
		t.Fatal("Matches(repo, \"repo-2\") = true, want false")
	}
}

func TestMatchesEmptySelectorMatchesNothing(t *testing.T) {
	repo := reposelector.Entry{ID: "repo-1", Name: "web"}
	for _, selector := range []string{"", "   "} {
		if reposelector.Matches(repo, selector) {
			t.Fatalf("Matches(repo, %q) = true, want false", selector)
		}
	}
}

func TestMatchesPathCleaning(t *testing.T) {
	repo := reposelector.Entry{ID: "repo-1", Path: "/srv/a/../web"}
	if !reposelector.Matches(repo, "/srv/web") {
		t.Fatal("Matches dirty Path against cleaned selector = false, want true")
	}
	if !reposelector.Matches(repo, "  /srv/web  ") {
		t.Fatal("Matches Path against padded selector = false, want true")
	}
}

func TestRepositorySelectorCanonicalizesOnlyPathFields(t *testing.T) {
	pathy := reposelector.Entry{ID: "repo-1", Name: "a/../web", Path: "/other"}
	if reposelector.Matches(pathy, "web") {
		t.Fatal("Matches canonicalized Name = true, want false: only path fields canonicalize")
	}
	withPath := reposelector.Entry{ID: "repo-2", Path: "a/../web"}
	if !reposelector.Matches(withPath, "web") {
		t.Fatal("Matches canonicalized Path = false, want true")
	}
}

func TestMatchesSymlinkSelector(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatalf("Mkdir = %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("Symlink = %v, cannot pin symlink matching here", err)
	}
	repo := reposelector.Entry{ID: "repo-1", Path: real}
	if !reposelector.Matches(repo, link) {
		t.Fatal("Matches real Path against symlink selector = false, want true")
	}
	if reposelector.Matches(repo, filepath.Join(dir, "missing")) {
		t.Fatal("Matches against missing selector = true, want false")
	}
}

func TestResolveSingleMatch(t *testing.T) {
	client := &fakeGetter{resp: reposelector.ListResponse{
		Repositories: []reposelector.Entry{
			{ID: "repo-1", Name: "web"},
			{ID: "repo-2", Name: "api"},
		},
		Total: 2,
	}}
	got, err := reposelector.Resolve(client, "web")
	if err != nil {
		t.Fatalf("Resolve = %v", err)
	}
	if got != "repo-1" {
		t.Fatalf("Resolve = %q, want repo-1", got)
	}
	if client.path != "/api/v0/repositories" {
		t.Fatalf("Resolve listed %q", client.path)
	}
}

func TestResolveNoMatch(t *testing.T) {
	client := &fakeGetter{resp: reposelector.ListResponse{
		Repositories: []reposelector.Entry{{ID: "repo-1", Name: "web"}},
	}}
	if _, err := reposelector.Resolve(client, "missing"); err == nil {
		t.Fatal("Resolve(missing) = nil error, want no-match error")
	}
}

func TestResolveAmbiguousNamesSortedIDs(t *testing.T) {
	client := &fakeGetter{resp: reposelector.ListResponse{
		Repositories: []reposelector.Entry{
			{ID: "repo-b", Path: "/srv/x"},
			{ID: "repo-a", Path: "/srv/x"},
		},
	}}
	_, err := reposelector.Resolve(client, "/srv/x")
	if err == nil {
		t.Fatal("Resolve(ambiguous) = nil error, want multiple-match error")
	}
	first := strings.Index(err.Error(), "repo-a")
	second := strings.Index(err.Error(), "repo-b")
	if first < 0 || second < 0 || first > second {
		t.Fatalf("Resolve error does not name sorted IDs: %v", err)
	}
}

func TestResolveDedupsSameID(t *testing.T) {
	client := &fakeGetter{resp: reposelector.ListResponse{
		Repositories: []reposelector.Entry{
			{ID: "repo-1", Name: "web"},
			{ID: "repo-1", Path: "web"},
		},
	}}
	got, err := reposelector.Resolve(client, "web")
	if err != nil {
		t.Fatalf("Resolve = %v", err)
	}
	if got != "repo-1" {
		t.Fatalf("Resolve = %q, want repo-1", got)
	}
}

func TestResolveNilClient(t *testing.T) {
	if _, err := reposelector.Resolve(nil, "web"); err == nil {
		t.Fatal("Resolve(nil, ...) = nil error, want wiring error")
	}
}

func TestResolveTransportError(t *testing.T) {
	boom := errors.New("transport down")
	client := &fakeGetter{err: boom}
	_, err := reposelector.Resolve(client, "web")
	if !errors.Is(err, boom) {
		t.Fatalf("Resolve = %v, want wrapped transport error", err)
	}
}
