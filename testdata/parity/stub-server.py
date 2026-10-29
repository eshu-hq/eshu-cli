#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
# Copyright (c) 2025-2026 eshu-hq
"""Canned API stub for Tier-1/Tier-2 CLI parity.

Usage:
  stub-server.py PORT LOGFILE

Logs "METHOD PATH AUTH=... BODY=..." per request. Hostile behavior is driven
by marker strings in the request body so the same binary invocation shape
exercises happy and hostile paths:

  HOSTILE_400 / HOSTILE_404 / HOSTILE_501 / HOSTILE_503 -> that HTTP status
  ENV_AMBIGUOUS / ENV_BUILDING / ENV_STALE / ENV_PARTIAL / ENV_DEGRADED /
  ENV_UNSUPPORTED / ENV_NOTFOUND -> 200 with an error-code envelope

Tier-2 routes serve pre-generated fixtures byte-for-byte. Each fixture holds
the exact response bytes the live S4 endpoint answers for the fixed parity
input (generated from the real core logic at the Tier-2 pin), so the thin
CLI's byte output can be compared against the core binary's. Fixture
selection keys on request-body markers documented below.

Everything else returns a canned 200 envelope echoing the request path.
"""
import json
import os
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 18080
LOG = sys.argv[2] if len(sys.argv) > 2 else "/tmp/parity-requests.log"
FIXTURES = os.path.join(os.path.dirname(os.path.abspath(__file__)), "fixtures")

STATUS_MARKERS = {
    "HOSTILE_400": 400,
    "HOSTILE_404": 404,
    "HOSTILE_501": 501,
    "HOSTILE_503": 503,
}

ENVELOPE_MARKERS = {
    "ENV_AMBIGUOUS": "ambiguous",
    "ENV_BUILDING": "index_building",
    "ENV_STALE": "stale",
    "ENV_PARTIAL": "partial",
    "ENV_DEGRADED": "capability_degraded",
    "ENV_UNSUPPORTED": "unsupported_capability",
    "ENV_NOTFOUND": "not_found",
}


class Handler(BaseHTTPRequestHandler):
    def _handle(self):
        length = int(self.headers.get("Content-Length", 0) or 0)
        body = self.rfile.read(length).decode("utf-8", "replace") if length else ""
        auth = self.headers.get("Authorization", "-")
        with open(LOG, "a", encoding="utf-8") as f:
            f.write(f"{self.command} {self.path} AUTH={auth} BODY={body}\n")

        for marker, status in STATUS_MARKERS.items():
            if marker in body or marker in self.path:
                return self._send(status, {"error": {"code": "parity", "message": marker}})

        for marker, code in ENVELOPE_MARKERS.items():
            if marker in body or marker in self.path:
                return self._send(
                    200,
                    {
                        "data": None,
                        "truth": {"parity": "stub"},
                        "error": {"code": code, "message": f"parity {marker}"},
                    },
                )

        # Tier-2: MCP tools/list answers the deployment's tool surface, decoded
        # by the thin CLI via apicontract.ToolDefinition.
        if self.path == "/mcp/message" and '"method":"tools/list"' in body.replace(" ", ""):
            return self._send(
                200,
                {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "result": {
                        "tools": [
                            {"name": "find_tool", "description": "find", "inputSchema": {"type": "object"}},
                            {"name": "call_tool", "description": "call", "inputSchema": {"type": "object"}},
                        ]
                    },
                },
            )

        # Tier-2: S4 CLI-support endpoints serve pre-generated fixtures. The
        # validate fixture keys on the bundle marker; the strict (require
        # public) run of the same public bundle answers the same valid
        # verdict. The compose fixture keys on the supply_chain request key;
        # the score fixture keys on the failing run id.
        if self.path == "/api/v0/report/bundle/capture":
            return self._send_file("capture-q.json")
        if self.path == "/api/v0/report/bundle/validate":
            # The CLI round-trips the bundle through encoding/json, which
            # compacts insignificant whitespace, so match spacing-insensitively.
            if '"parity-case":"fail"' in body.replace(" ", ""):
                return self._send_file("validate-fail.json")
            return self._send_file("validate-pass.json")
        if self.path == "/api/v0/service/report/compose":
            if '"supply_chain"' in body:
                return self._send_file("compose-supply.json")
            return self._send_file("compose-story.json")
        if self.path == "/api/v0/answer-quality/score":
            if '"parity-fail"' in body:
                return self._send_file("score-fail.json")
            return self._send_file("score-pass.json")

        # Tier-2: component inventory/diagnostics serve fixed extension rows.
        if self.path.startswith("/api/v0/component-extensions"):
            if self.path.endswith("/diagnostics"):
                return self._send_file("component-diagnostics.json")
            return self._send_file("component-inventory.json")

        self._send(
            200,
            {
                "data": {"parity": "ok", "path": self.path, "token": "SECRET-VALUE"},
                "truth": {"parity": "stub", "token": "SECRET-VALUE"},
            },
        )

    do_GET = do_POST = do_PUT = do_DELETE = _handle

    def _send(self, status, obj):
        payload = json.dumps(obj).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def _send_file(self, name):
        # Fixture bytes go out untouched: re-encoding through json would
        # reorder nothing but could re-escape strings (notably <>&), breaking
        # the byte parity the fixtures exist to prove.
        with open(os.path.join(FIXTURES, name), "rb") as f:
            payload = f.read()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    HTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
