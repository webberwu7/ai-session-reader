# Project Context

AI Session Reader is a Go local CLI for searching Claude Code and Codex
sessions and loading compact transcripts into another AI session.

Read README.md for the current scope and roadmap. The initial CLI is implemented in cmd/ai-session; provider parsing lives in internal/session.
Run go test -race ./... and go vet ./... before handing off changes.

## Development principles

- Keep provider parsing behind adapters and shared functionality provider-neutral.
- First targets: Claude Code and Codex CLI/Desktop local transcripts.
- Google Antigravity is a future research target; do not invent its storage format.
- Search original content, but compact large tool bodies for context inheritance.
- Preserve provenance and expose unsupported or skipped records via diagnostics.
- Use synthetic test fixtures; never commit actual conversations or credentials.
- Implementation language: Go 1.24+, standard library only. Licensing remains undecided.

## Workspace instructions

When working inside Webber's workspace, read the root AI_CONTEXT.md before changes.
