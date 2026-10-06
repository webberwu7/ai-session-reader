# AI Session Reader

A local-first CLI for searching and reading AI coding sessions across providers.

**Status: project planning. No working CLI or provider adapter is implemented yet.**

## Goal

Find past work across Claude Code and Codex, then load the conversation into a new
session with less tool-output noise. Preserve conversation text, summarize tool
calls, and allow individual calls to be expanded when their details matter.

## Planned provider support

| Provider | Status | Intended input |
| --- | --- | --- |
| Claude Code | Initial milestone | Local session transcripts |
| OpenAI Codex CLI / Desktop | Initial milestone | Local rollout transcripts |
| Google Antigravity | Future research | Format and access method to be verified |
| ChatGPT | Future research | User-provided data exports |

Google Antigravity support is a roadmap item, not an existing capability. Its
storage format and supported export mechanisms must be investigated before an
adapter can be designed. Ordinary ChatGPT conversations are a separate source
from Codex transcripts.

## Proposed CLI

These commands describe the intended interface; they are not runnable yet.

```sh
ai-session list
ai-session search "login error"
ai-session search "login error" --agent codex
ai-session read <session-id>
ai-session inherit <session-id>
ai-session expand <session-id> <tool-id>
ai-session stats <session-id>
ai-session doctor
```

Search should inspect original message and tool content. Compact reading should
preserve human and assistant conversation text while replacing large tool bodies
with short references. `inherit` should paginate compact transcripts and track
progress; `expand` should retrieve original tool input and output.

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

This repository currently contains original planning documents only. Any future
reuse of upstream code must preserve its applicable license and attribution.
