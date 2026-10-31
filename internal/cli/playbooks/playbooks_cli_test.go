// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package playbooks_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/eshu-hq/eshu-cli/internal/cli/playbooks"
)

type fakeEnvelopeClient struct {
	getPath string
	getErr  error
	list    playbooks.ListEnvelope

	postPath string
	postBody any
	postErr  error
	resolve  playbooks.ResolveEnvelope
}

func (f *fakeEnvelopeClient) GetEnvelope(path string, result any) error {
	f.getPath = path
	if f.getErr != nil {
		return f.getErr
	}
	if env, ok := result.(*playbooks.ListEnvelope); ok {
		*env = f.list
	}
	return nil
}

func (f *fakeEnvelopeClient) PostEnvelope(path string, body, result any) error {
	f.postPath = path
	f.postBody = body
	if f.postErr != nil {
		return f.postErr
	}
	if env, ok := result.(*playbooks.ResolveEnvelope); ok {
		*env = f.resolve
	}
	return nil
}

func TestParseInputsTable(t *testing.T) {
	got, err := playbooks.ParseInputs([]string{"a=1", " b = 2 ", "empty="})
	if err != nil {
		t.Fatalf("ParseInputs = %v", err)
	}
	want := map[string]string{"a": "1", "b": "2", "empty": ""}
	if len(got) != len(want) {
		t.Fatalf("ParseInputs = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("ParseInputs[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestParseInputsEmpty(t *testing.T) {
	got, err := playbooks.ParseInputs(nil)
	if err != nil {
		t.Fatalf("ParseInputs(nil) = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ParseInputs(nil) = %v, want empty", got)
	}
}

func TestParseInputsRejects(t *testing.T) {
	for _, raw := range []string{"novalue", "=nokey", "   =blank"} {
		if _, err := playbooks.ParseInputs([]string{raw}); err == nil {
			t.Fatalf("ParseInputs(%q) = nil error, want key=value error", raw)
		}
	}
}

func TestRunListNilClientWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	if err := playbooks.RunList(&buf, nil); err == nil {
		t.Fatal("RunList(nil) = nil error, want wiring error")
	}
	if buf.Len() != 0 {
		t.Fatalf("RunList(nil) wrote %q, want nothing", buf.String())
	}
}

func TestRunListWritesIndentedEnvelope(t *testing.T) {
	client := &fakeEnvelopeClient{
		list: playbooks.ListEnvelope{
			Data:  map[string]any{"items": []any{"pb-a"}},
			Truth: map[string]any{"source": "live"},
		},
	}
	var buf bytes.Buffer
	if err := playbooks.RunList(&buf, client); err != nil {
		t.Fatalf("RunList = %v", err)
	}
	if client.getPath != "/api/v0/query-playbooks?view=full&limit=200" {
		t.Fatalf("RunList requested %q", client.getPath)
	}
	out := buf.String()
	if !strings.HasSuffix(out, "}\n") || !strings.Contains(out, "\n  \"data\"") {
		t.Fatalf("RunList wrote non-indented JSON: %q", out)
	}
	if !strings.Contains(out, "pb-a") {
		t.Fatalf("RunList dropped envelope data: %q", out)
	}
}

func TestRunListTransportErrorWritesNothing(t *testing.T) {
	boom := errors.New("transport down")
	var buf bytes.Buffer
	if err := playbooks.RunList(&buf, &fakeEnvelopeClient{getErr: boom}); err != boom {
		t.Fatalf("RunList = %v, want transport error", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("RunList wrote %q on transport failure, want nothing", buf.String())
	}
}

func TestRunResolveTrimsIDAndPassesInputs(t *testing.T) {
	client := &fakeEnvelopeClient{}
	var buf bytes.Buffer
	opts := playbooks.ResolveOptions{
		PlaybookID: "  pb-9  ",
		Inputs:     map[string]string{"env": "prod"},
	}
	if err := playbooks.RunResolve(&buf, client, opts); err != nil {
		t.Fatalf("RunResolve = %v", err)
	}
	if client.postPath != "/api/v0/query-playbooks/resolve" {
		t.Fatalf("RunResolve posted to %q", client.postPath)
	}
	body, ok := client.postBody.(map[string]any)
	if !ok {
		t.Fatalf("RunResolve posted body type %T", client.postBody)
	}
	if body["playbook_id"] != "pb-9" {
		t.Fatalf("RunResolve posted playbook_id %v, want trimmed pb-9", body["playbook_id"])
	}
	inputs, ok := body["inputs"].(map[string]string)
	if !ok || inputs["env"] != "prod" {
		t.Fatalf("RunResolve posted inputs %v", body["inputs"])
	}
	if !strings.HasSuffix(buf.String(), "}\n") {
		t.Fatalf("RunResolve wrote non-envelope output: %q", buf.String())
	}
}

func TestRunResolveNilClientWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	opts := playbooks.ResolveOptions{PlaybookID: "pb-9"}
	if err := playbooks.RunResolve(&buf, nil, opts); err == nil {
		t.Fatal("RunResolve(nil) = nil error, want wiring error")
	}
	if buf.Len() != 0 {
		t.Fatalf("RunResolve(nil) wrote %q, want nothing", buf.String())
	}
}

func TestRunResolveTransportErrorWritesNothing(t *testing.T) {
	boom := errors.New("transport down")
	var buf bytes.Buffer
	opts := playbooks.ResolveOptions{PlaybookID: "pb-9"}
	if err := playbooks.RunResolve(&buf, &fakeEnvelopeClient{postErr: boom}, opts); err != boom {
		t.Fatalf("RunResolve = %v, want transport error", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("RunResolve wrote %q on transport failure, want nothing", buf.String())
	}
}

func TestWriteJSONKeepsHTMLVerbatim(t *testing.T) {
	client := &fakeEnvelopeClient{
		list: playbooks.ListEnvelope{
			Data: map[string]any{"url": "https://x.example/cb?a=1&b=2"},
		},
	}
	var buf bytes.Buffer
	if err := playbooks.RunList(&buf, client); err != nil {
		t.Fatalf("RunList = %v", err)
	}
	if !strings.Contains(buf.String(), "a=1&b=2") {
		t.Fatalf("RunList escaped HTML in JSON: %q", buf.String())
	}
}
