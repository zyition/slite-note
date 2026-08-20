# Backlog

Queued feature requests, kept out of the README until they ship. The README
intentionally shows no roadmap; pick from here when planning the next release.

- Linux port (design shape reserved in the platform layer — ADR-0006; settle
  details in a grilling round when picked up)
- `scoop` / `winget` manifests

## Rejected

- Slide in/out window animation (the "slide" in slite-note): conflicts with
  the `WS_EX_LAYERED` opacity path (`AnimateWindow` is a no-op on layered
  windows), a frontend CSS animation would only animate content while the
  window itself pops in/out instantly, and frame-by-frame `SetWindowPos`
  animation fights the opacity re-apply hooks and delays hide. Not worth it.
