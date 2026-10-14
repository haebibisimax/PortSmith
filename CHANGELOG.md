# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-08-31

### Documentation

- Describe the fastest way to exercise the pipeline end to end.


### Added
- **Trace format v1** — documented line-based, base64-framed trace format shared
  by both tools (`docs/FORMAT.md`).
- **`portcap` (Go)**:
  - `capture` — transparent TCP proxy recording both directions of a session.
  - `normalize` — convert `> / <` raw session logs into canonical traces.
  - `stats` — counts, byte totals, session counts, and protocol breakdown.
- **`portsmith-replay` (Rust)**:
  - `infer` — dependency-free schema inference (text/binary classification,
    framing/terminator detection, leading-token distribution, length stats).
  - `replay` — replay captured requests against a live TCP target with optional
    timing preservation and configurable read timeouts.
  - `cat` — pretty-print a trace with control characters escaped.
  - Self-contained standard-base64 encoder/decoder (no external crates).
- Sample Redis and HTTP session logs and generated traces under `samples/`.
- Go unit tests (trace round-trip and decoder validation) and Rust integration
  tests (base64 known-answer, trace parsing, schema inference).
- `Makefile`, GitHub Actions CI (`.github/workflows/ci.yml`), MIT `LICENSE`.

[1.0.0]: https://example.com/portsmith/releases/tag/v1.0.0
