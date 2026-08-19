# Contributing to slite-note

Thanks for taking the time to contribute! slite-note is a small, focused
project — read the whole issue/PR before jumping in.

## Ground rules

- **Scope discipline**: slite-note is intentionally minimal. Large feature
  proposals should be discussed in an issue first (the roadmap lists known
  candidates).
- **Windows only for now**: the shell relies on Win32 (tray, global hotkey,
  window placement). macOS/Linux ports are tracked separately.
- **Zero telemetry stays zero**: no analytics, no network calls at runtime.

## Getting started

1. Fork and clone the repo.
2. Follow the [Building] section of the README.
3. Read [AGENTS.md](./AGENTS.md) — it documents the toolchain, mirrors and
   quality gate.

## Quality gate (must pass before submitting)

```powershell
go vet .                          # main package
cd frontend && pnpm exec tsc --noEmit
```

- No automated e2e tests exist by design; the maintainer manually verifies
  native features. For UI-only changes you can develop in the browser
  fallback (`cd frontend && pnpm dev`).

## Submitting changes

- Keep PRs small and focused — one change per PR.
- **Commit messages must follow Conventional Commits** — see
  [docs/commits.md](./docs/commits.md) for the type list, scope conventions
  and AI-generated commit rules. The changelog is generated from commit
  history, so a non-conventional commit disappears from it.
- Update the docs that describe what you changed (README, AGENTS.md,
  THIRD_PARTY_NOTICES.md when adding dependencies).
- Reference the issue your PR closes (`Closes #12`).
- For non-trivial design decisions, propose an ADR in `docs/adr/`
  (see existing records for the format).

## Code of conduct

Be respectful and constructive. Harassment of any kind is not tolerated.

[Building]: https://github.com/zyition/slite-note#building
