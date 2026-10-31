// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package doctor

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// failWriter is an io.Writer that always fails. Run discards every write
// error by design (each write is `_, _ =`, and Run always returns nil
// because the checks are advisory), so this pins the swallow contract the
// same way the failWriter in freshness_render_cli_test.go pins propagation
// there -- here the assertion is inverted on purpose.
type failWriter struct{}

func (failWriter) Write(p []byte) (int, error) { return 0, errors.New("no space") }

// dripWriter succeeds for the first failAfter writes, then fails. Stepping
// failAfter from 0 upward makes each successive write in the report sequence
// the failing one, proving no mid-sequence write failure escapes as an
// error. There is no write-error arm to cover, only the advisory always-nil
// contract to hold at every position.
type dripWriter struct {
	failAfter int
	writes    int
}

func (w *dripWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > w.failAfter {
		return 0, errors.New("no space")
	}
	return len(p), nil
}

// statusTransport serves one canned status without touching the network, so
// the non-200 branch does not depend on a live server's failure mode.
type statusTransport struct{ code int }

func (s statusTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: s.code,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

// healthyDeps describes a machine where everything is right: a real config
// directory and settings file (resolved through the real os.Stat default),
// every service binary on PATH, and both datastores configured. Stat and
// HTTPClient are deliberately nil so the os.Stat and bounded-client defaults
// are exercised rather than stubbed.
func healthyDeps(t *testing.T, apiBaseURL string) Deps {
	t.Helper()

	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("x=1\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v, want nil", err)
	}
	return Deps{
		ConfigDir:   dir,
		EnvFilePath: envFile,
		APIBaseURL:  apiBaseURL,
		Neo4jURI:    "bolt://neo4j@graph.example.com:7687",
		PostgresDSN: "postgres://eshu@db.example.com:5432/eshu",
		LookPath:    func(bin string) (string, error) { return filepath.Join("/usr/bin", bin), nil },
	}
}

// TestRunReportsAHealthyMachine covers the blocks the all-broken fixtures
// never reach: the nil-HTTPClient default, the LookPath-found branch, the
// 200-healthy branch, and the response-close defer. The probe-path assertion
// keeps healthURL wired into Run rather than tested only in isolation.
func TestRunReportsAHealthyMachine(t *testing.T) {
	var probedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	if err := Run(&buf, healthyDeps(t, srv.URL)); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if probedPath != "/health" {
		t.Errorf("probe path = %q, want /health", probedPath)
	}

	out := buf.String()
	for _, want := range []string{
		"Config directory exists",
		"Config file exists",
		"API healthy at " + srv.URL,
		"Neo4j URI configured",
		"Postgres DSN configured",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q\n%s", want, out)
		}
	}
	for _, bin := range serviceBinaries {
		want := "[ok] " + bin + " found: " + filepath.Join("/usr/bin", bin)
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q\n%s", want, out)
		}
	}
}

// TestRunReportsADegradedAPIStatus covers the last zero-hit block: a probe
// that answers with a non-200 status. A degraded API is still advisory, so
// the report names the status and Run still returns nil.
func TestRunReportsADegradedAPIStatus(t *testing.T) {
	deps := healthyDeps(t, "http://api.example.com")
	deps.HTTPClient = &http.Client{
		Timeout:   time.Second,
		Transport: statusTransport{code: http.StatusInternalServerError},
	}

	var buf bytes.Buffer
	if err := Run(&buf, deps); err != nil {
		t.Fatalf("Run() error = %v, want nil (checks are advisory)", err)
	}
	if !strings.Contains(buf.String(), "API returned status 500") {
		t.Errorf("report missing degraded-API line\n%s", buf.String())
	}
}

// countDoctorWrites measures how many writes a healthy report performs, so
// the stepping test below cannot pass vacuously on a zero-write run.
func countDoctorWrites(t *testing.T, deps Deps) int {
	t.Helper()

	w := &dripWriter{failAfter: 1 << 30}
	if err := Run(w, deps); err != nil {
		t.Fatalf("Run(dripWriter) error = %v, want nil (writes are advisory)", err)
	}
	return w.writes
}

// offlineDeps is a fully stubbed healthy machine with no network: the API
// probe fails closed through errorTransport, so the write-stepping tests are
// deterministic on any host.
func offlineDeps(t *testing.T) Deps {
	t.Helper()

	deps := healthyDeps(t, "http://127.0.0.1:9")
	deps.HTTPClient = &http.Client{Timeout: time.Second, Transport: errorTransport{}}
	return deps
}

// TestRunIgnoresWriteErrors pins the advisory contract at the writer: even a
// writer that fails every write still yields a nil error, because each write
// in Run is explicitly discarded.
func TestRunIgnoresWriteErrors(t *testing.T) {
	if err := Run(failWriter{}, offlineDeps(t)); err != nil {
		t.Fatalf("Run(failWriter) = %v, want nil (every write is explicitly discarded)", err)
	}
}

// TestEveryWriteFailureStaysSilent steps the failure across every write in
// the sequence. A render that propagated a mid-sequence write error would
// return non-nil here; Run must stay nil at every position.
func TestEveryWriteFailureStaysSilent(t *testing.T) {
	deps := offlineDeps(t)
	total := countDoctorWrites(t, deps)
	if total == 0 {
		t.Fatal("healthy report performed no writes; the loop below proves nothing")
	}
	for i := 0; i < total; i++ {
		w := &dripWriter{failAfter: i}
		if err := Run(w, deps); err != nil {
			t.Errorf("Run() with write %d failing = %v, want nil (advisory: errors are discarded)", i+1, err)
		}
	}
}
