#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
# Copyright (c) 2025-2026 eshu-hq
"""Canned API stub for Tier-1 CLI parity.

Usage:
  stub-server.py PORT LOGFILE

Logs "METHOD PATH AUTH=... BODY=..." per request. Hostile behavior is driven
by marker strings in the request body so the same binary invocation shape
exercises happy and hostile paths:

  HOSTILE_400 / HOSTILE_404 / HOSTILE_501 / HOSTILE_503 -> that HTTP status
  ENV_AMBIGUOUS / ENV_BUILDING / ENV_STALE / ENV_PARTIAL / ENV_DEGRADED /
  ENV_UNSUPPORTED / ENV_NOTFOUND -> 200 with an error-code envelope

Everything else returns a canned 200 envelope echoing the request path.
"""
import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 18080
LOG = sys.argv[2] if len(sys.argv) > 2 else "/tmp/parity-requests.log"

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

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    HTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
