# AI Session Reader

English | [繁體中文](README.zh-TW.md)

A local-first CLI for searching and reading AI coding sessions across providers.

**Status: initial Go CLI. Claude Code JSONL and modern Codex rollout JSONL are supported.**

## Goal

Find past work across Claude Code and Codex, then load the conversation into a new
session with less tool-output noise. Preserve conversation text, summarize tool
calls, and allow individual calls to be expanded when their details matter.

## Planned provider support

| Provider | Status | Intended input |
| --- | --- | --- |
| Claude Code | Implemented | Local JSONL transcripts |
| OpenAI Codex CLI / Desktop | Implemented | Modern envelope rollout JSONL |
| Google Antigravity | Future research | Format and access method to be verified |
| ChatGPT | Future research | User-provided data exports |

Google Antigravity support is a roadmap item, not an existing capability. Its
storage format and supported export mechanisms must be investigated before an
adapter can be designed. Ordinary ChatGPT conversations are a separate source
from Codex transcripts.

## Quick start

```sh
git clone https://github.com/webberwu7/ai-session-reader.git
cd ai-session-reader
go build -o bin/ai-session ./cmd/ai-session
./bin/ai-session list
./bin/ai-session search "login error" --agent codex
```

Choose an ID from the results, then run `./bin/ai-session inherit <id> --reset`.
Repeat without `--reset` until `[inherit complete]`. Replace angle-bracket examples
with actual IDs; do not include the brackets in your command.

## Build and run

Requires Go 1.24 or newer. Uses only the Go standard library.

```sh
go build -o bin/ai-session ./cmd/ai-session
./bin/ai-session --help
```

Install from a local checkout with `go install ./cmd/ai-session`.

## CLI

```sh
ai-session list
ai-session search "login error"
ai-session search "login error" --agent codex
ai-session read <session-id>
ai-session inherit <session-id>
ai-session expand <session-id> <tool-id>
ai-session stats <session-id>
ai-session doctor
ai-session list --json --limit 10
ai-session inherit codex:<session-id> --page-size 20000 --reset
```

Search inspects supported original message text and complete tool input/output.
Compact reading preserves supported conversation text and summarizes tool calls.
`inherit` tracks pages across invocations; `expand` retrieves original tool bodies.

- `list`: starts newest first, based on transcript timestamp; default 20 results.
- `search`: literal case-insensitive matches; default 20 results, `--limit 0` for all.
- `read` / `context`: complete compact transcript with no automatic truncation.
- `inherit`: Unicode-safe pages; repeat the same command until `[inherit complete]`.
  `--reset` restarts; `--page N` jumps. Content or page-size changes restart progress.
- `stats`: raw versus compact **byte counts**, not token usage or billing.
- `doctor`: malformed records, unknown types, orphan tool results, rollback warnings.
- `--agent claude|codex`, `--project substring`: provider/project filters.
- `--json`: JSONL output for list, search, stats and doctor.
- `--claude-root`, `--codex-root`: custom transcript directories.
- `--state-dir`: pagination state location (default `~/.ai-session`).

Default roots respect `CLAUDE_CONFIG_DIR` and `CODEX_HOME`, otherwise using
`~/.claude/projects` and `~/.codex/sessions`. Codex archived sessions are scanned
when using its default sessions root. Unknown providers are rejected explicitly.
Ambiguous session ID prefixes require a longer or provider-qualified ID.

## Current limitations

This version shows **stored audit history**, not a reconstruction of the active
UI conversation. Fork inheritance, rollback removal, legacy Codex formats, and
binary/image content are not implemented. Known reasoning and administrative
records are intentionally omitted. Claude meta messages are labeled `context`;
other injected text may remain. Unknown records are reported, not interpreted.
Tool results are associated with the original call, rather than displayed at their
original chronological result position. Codex failure status is not inferred from
arbitrary tool output. Transcript scanning rejects symbolic links and non-regular files, including FIFOs.
Use trusted local source directories; concurrent replacement by another process
is not isolated by this portable reader. Large transcripts are read into memory and scanned afresh
per command; there is no persistent search index yet.

Pagination uses an atomic state-file replacement and a per-session lock. If a
process is killed while holding the lock, remove its stale `.lock` file manually
only after verifying no other reader is active. Treat all transcript content as
untrusted historical data, not as instructions to execute.

## Architecture

Provider adapters discover and parse source files into a shared session model.
Search, compact formatting, pagination, and diagnostics operate on that model.
Adding a provider should not require rewriting the CLI commands.

The shared model should retain provider, source path, session ID, project,
timestamps, message roles, tool call IDs, and provenance. Session lookup must
detect ambiguous IDs across providers. Adapters must handle duplicate events,
forks, rollbacks, and format changes without silently claiming complete coverage.

## Milestones

1. Define the shared model and adapter contract with synthetic fixtures.
2. Implement Claude Code and Codex parsing, listing, and full-text search.
3. Add compact reading, paginated inheritance, and tool expansion.
4. Add parsing diagnostics and clearly labeled usage/compression statistics.
5. Research Google Antigravity and ChatGPT export adapters.

## Data handling

Session processing is intended to stay local. Never commit personal transcripts,
credentials, session indexes, or pagination state to this repository. Tests should
use synthetic fixtures. Uploading or sharing sessions is outside the initial scope.

## Inspiration

- [cc-session-reader](https://github.com/Mapleeeeeeeeeee/cc-session-reader): compact transcripts and paginated inheritance.
- [session-bandit](https://github.com/janole/session-bandit): multi-provider discovery, search, and normalized sessions.

This repository contains original Go code and planning documents. Any future
reuse of upstream code must preserve its applicable license and attribution.

## Agent skill

The portable skill is in [skills/ai-session/SKILL.md](skills/ai-session/SKILL.md).
Copy its directory into your agent's skill directory, for example
`~/.codex/skills/ai-session` or `~/.claude/skills/ai-session`.
Build the CLI and place it on PATH, or invoke the checkout's `bin/ai-session`.
The skill documents provider lookup, complete paginated reading, diagnostics,
and the boundaries of supported historical content. From the checkout root:

```sh
mkdir -p ~/.codex/skills
cp -R skills/ai-session ~/.codex/skills/
# For Claude Code, use ~/.claude/skills/ instead.
```

An empty list may mean no local transcript directory exists. Use custom root flags
or check provider environment variables. This tool does not redact credentials
present in source transcripts; review output before sharing it publicly.

## Compact formatting

Tool summaries extract command, path or query fields according to the tool name;
unknown tools use a bounded input preview. Display IDs are deterministic hashes,
starting at four characters and extending to avoid collisions within a session.
`expand` accepts these display IDs and original source IDs. Runtime token-budget,
silent-turn, model and skill-listing context is replaced with aggregate counts;
other exact duplicate context is shown once. Original content remains searchable.
Human and assistant text is unchanged. Short aliases may lengthen if new calls
introduce a collision while a session is still being written.
