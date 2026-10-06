# Project Context

AI Session Reader is a planned local CLI for searching Claude Code and Codex
sessions and loading compact transcripts into another AI session.

Read README.md for the current scope and roadmap. No CLI is implemented yet.

## Development principles

- Keep provider parsing behind adapters and shared functionality provider-neutral.
- First targets: Claude Code and Codex CLI/Desktop local transcripts.
- Google Antigravity is a future research target; do not invent its storage format.
- Search original content, but compact large tool bodies for context inheritance.
- Preserve provenance and expose unsupported or skipped records via diagnostics.
- Use synthetic test fixtures; never commit actual conversations or credentials.
- Implementation language and licensing remain undecided.

## Workspace instructions

When working inside Webber's workspace, read the root AI_CONTEXT.md before changes.
