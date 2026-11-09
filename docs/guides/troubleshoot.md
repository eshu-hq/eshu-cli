# Troubleshoot eshu-cli

The common messages, what they're trying to tell you, and the fix.
Start at the top — the first two cover most bad days.

## `request failed`

The URL is wrong or nothing is listening there. Check
`ESHU_SERVICE_URL`, your saved config (`eshu config show`), and
whether a `--service-url` flag or profile is quietly winning. The
resolution order in [Auth setup](auth-setup.md) explains why.

## `API error 401`

Your key is missing or doesn't match what the service expects. Confirm
`ESHU_API_KEY` (or the saved value) is the one the service was started
with. Remember the key is only sent when set, so an empty value looks
exactly like no key at all.

## A `STUB:` message

That family still lives in the core binary. The message tells you
where to go. It's not a failure in your setup. Same for a "removed"
message from commands like `delete` or `clean`: follow the pointer to
the admin flow it names.

## `eshu index` can't find its helper

`eshu-bootstrap-index binary not found in PATH` means indexing hands
over to that separate binary (see
[Boundaries](../concepts/boundaries.md)). The error prints the build
command (`cd go && go build -o bin/ ./cmd/bootstrap-index/` in the
core tree). Build it, put it on your `PATH`, and retry.

## `eshu install nornicdb` refuses to run

Without `--from`, that's deliberate while the no-argument installer
policy is in force. Point it at a local binary, archive, or package,
or at a release URL with `--sha256`. Add `--force` when you're
replacing a managed binary that's already there.

## `eshu config set` fails with a path error

The message names the directory or file it couldn't write under
`ESHU_HOME` (default `~/.eshu`). Fix permissions there. The file is
written `0600`, so a config owned by someone else will bounce.

## `eshu mcp setup --write` has no safe target

Some clients don't have a merge target we trust yet. Print the snippet
(drop `--write`) and paste it into the file the message names.
`--target` can also point the write somewhere explicit.

## Everything works but slowly

Or calls time out at 30 seconds. Raise
`ESHU_REMOTE_TIMEOUT_SECONDS` while you work out whether the service
or the network is the slow part.

## One last trick: read the exit code

In scripts, check the exit code before you parse output. A `4` means
the index is building or stale, which often just means "wait and
retry", while a `2` means the request itself was wrong. The full table
is in [Reference: configuration](../reference/configuration.md).
