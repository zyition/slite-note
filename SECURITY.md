# Security Policy

## Scope

slite-note stores user data locally (`%APPDATA%\slite\`) and performs **no
network communication at runtime** — no telemetry, no analytics, no update
checker. There is no server-side component.

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities. Report
privately via one of:

- GitHub **private vulnerability reporting** on the repository
  (Security → *Report a vulnerability*), or
- email to the maintainer (see repository profile).

Include: affected version(s), a minimal reproduction, and your suggested fix
if you have one.

## What to expect

- We'll acknowledge within 5 business days.
- We'll work with you on a coordinated fix before public disclosure.

## Notes for users

- The app is fully offline; no data leaves your machine.
- Notes are stored in plain text in a single JSON file — protect your
  `%APPDATA%\slite\` directory as you would any local document.
