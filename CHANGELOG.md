# Changelog

All notable changes to this project are documented here, in
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format.
Versioning follows [Semantic Versioning](https://semver.org/) once the
first tagged release ships.

## [Unreleased]

## [0.1.0] - 2026-09-04

### Added
- `InsertLines`, `DeleteLines`, `EraseChars`, `SetScrollRegion`
  (DECSTBM), `ResetScrollRegion`.
- Structured `DeviceAttributesEvent` parsing — `Secondary` and `Params`
  alongside the raw reply string.
- Synchronized-output helpers on `Terminal`: `BeginSyncUpdate`,
  `EndSyncUpdate`, `SyncUpdate`.
- `ColorEvent` and OSC 10/11 reply parsing. `QueryForegroundColor` /
  `QueryBackgroundColor` on `Terminal` now actually resolve instead of
  sending a query nothing ever answers.

### Fixed
- Every DEC private-mode CSI sequence (alt screen, mouse tracking,
  bracketed paste, focus reporting, cursor hide/show, synchronized
  output) was broken — `'?'` in a non-final CSI parameter position
  rendered as its decimal codepoint instead of the literal character,
  so e.g. alt-screen-enable came out as `\x1b[63;1049h`, which no
  terminal recognizes.
- `Modifier` bitmask constants: `ModShift` was `2`, not `1` — an `iota`
  off-by-one wasted bit 0.
- Ctrl-C and Ctrl-D produced identical events (`Rune: 0`, no way to
  tell them apart). Every Ctrl+letter now maps consistently to its
  base rune with `ModCtrl` set.
- `ReadEvent` could miss a resize delivered while nothing was being
  typed — a blocked `Read` isn't reliably interrupted by SIGWINCH
  under Go's runtime-integrated poller. Input now flows through a
  background reader goroutine so `ReadEvent`/`TryReadEvent` can select
  across resize and input instead of sequentially blocking then
  checking.
- `TryReadEvent` never actually read from the terminal — it only
  checked already-buffered state, so a standalone poll loop never saw
  new input at all.
- `SetTitle`/OSC arguments are sanitized to strip control bytes,
  closing an escape-sequence injection vector via untrusted title text.
- `atoi` no longer silently overflows via wraparound on a
  pathologically long digit run from a corrupted or hostile terminal
  reply.

[Unreleased]: https://github.com/dominionthedev/leak/compare/v0.1.0...main
[0.1.0]: https://github.com/dominionthedev/leak/releases/tag/v0.1.0
