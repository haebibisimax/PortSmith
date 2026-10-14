<!-- portsmith — protocol cartographer's field manual -->
<p align="center">
  <img src="docs/assets/hero.svg" alt="portsmith — a protocol cartographer's field manual: an oscilloscope tracing packets through capture, normalize, infer and replay" width="100%">
</p>

<h1 align="center">portsmith</h1>

<p align="center">
  <strong>Protocol discovery &amp; replay for line-based network traffic.</strong><br>
  <em>Put an unknown wire protocol on the bench, watch its waveform, chart its schema, and replay it.</em>
</p>

<p align="center">
  <code>capture</code> &#8594; <code>normalize</code> &#8594; <code>infer</code> &#8594; <code>replay</code> &#183;
  <code>Go</code> + <code>Rust</code> &#183; standard libraries only &#183; MIT
</p>

---

## Field manual

You have a TCP service with no spec — an undocumented daemon, a vendor box that
speaks its own dialect, a Redis-flavoured thing you want to regression-test. You
have a socket, not a document.

**portsmith is the bench kit for that socket.** It treats an opaque protocol the
way an electronics workshop treats an unknown signal: clip a probe onto the wire
(`capture`), tidy the recording into a clean trace (`normalize`), read the
waveform and chart its structure (`infer`), then drive the same stimulus back
into a live device (`replay`). No protocol-specific code, no dependency tree —
two small binaries, each in the language that suits its job, exchanging one
documented text format.

> **Not** a Wireshark, a fuzzer, or a full reverse-engineering suite. portsmith
> is a focused, auditable toolchain for line-oriented request/response traffic
> you can read end-to-end in an afternoon.

---

## Table of contents

- [Why portsmith](#why-portsmith)
- [The two instruments](#the-two-instruments)
- [Install &amp; build](#install--build)
- [Bench session (annotated transcript)](#bench-session-annotated-transcript)
- [The trace format, walked field by field](#the-trace-format-walked-field-by-field)
- [Architecture](#architecture)
- [Trace flow](#trace-flow)
- [Command reference](#command-reference)
- [Workflows](#workflows)
- [Use cases](#use-cases)
- [How inference actually works](#how-inference-actually-works)
- [How replay actually works](#how-replay-actually-works)
- [Design choices](#design-choices)
- [Comparison](#comparison)
- [Limitations](#limitations)
- [Troubleshooting](#troubleshooting)
- [Repository layout](#repository-layout)
- [Roadmap](#roadmap)
- [License](#license)

---

## Why portsmith

Most protocol tooling assumes you already know the protocol. portsmith assumes
you do not, and optimizes for the *discovery loop*:

- **Two languages, deliberately.** Capture and normalization are I/O-and-goroutine
  heavy — Go's home turf (`portcap`). Inference and replay want a tight,
  allocation-conscious core with strong pattern matching — Rust
  (`portsmith-replay`). Neither reaches for a third-party crate or module.
- **One contract between them.** The `*.trace` file is the *only* coupling. Both
  sides implement the same v1 grammar against their standard libraries, including
  a hand-rolled standard-base64 codec on the Rust side.
- **Everything is inspectable.** Traces are UTF-8 text; payloads are base64 so a
  single grep-able line survives binary data and embedded NULs. Schema reports and
  replay output are plain text, made for humans and pipes alike.

---

## The two instruments

| Instrument | Language | Role on the bench | Subcommands |
|------------|----------|-------------------|-------------|
| **`portcap`** | Go | The probe &amp; recorder. Sits in front of a TCP service as a transparent proxy and records both directions; also normalizes loose logs and summarizes traces. | `capture`, `normalize`, `stats`, `version` |
| **`portsmith-replay`** | Rust | The scope &amp; signal generator. Reads a trace, infers the protocol's structure, pretty-prints it, and replays captured requests against a live target. | `infer`, `replay`, `cat`, `version` |

Both tools report version `1.0.0`.

---

## Install &amp; build

Prerequisites: **Go 1.24** and a **stable Rust toolchain** (`cargo`).

```sh
make build        # builds the Go binary (bin/portcap) and the Rust release binary
make test         # runs `go test ./...` and `cargo test`
```

Prefer to drive each toolchain yourself:

```sh
cd go   && go build -o ../bin/portcap ./cmd/portcap
cd rust && cargo build --release        # target/release/portsmith-replay
```

Other Makefile targets that mirror CI:

```sh
make fmt     # go fmt ./...            + cargo fmt
make vet     # go vet ./...            + cargo check
make demo    # regenerate samples/redis.trace, then infer it
make clean   # remove bin/ and cargo artifacts
```

CI (`.github/workflows/ci.yml`) runs three jobs: **Go** (gofmt check, `go vet`,
build, `go test -race`), **Rust** (`cargo fmt --check`, `clippy -D warnings`,
release build, `cargo test`), and a **cross-language interop** job that
normalizes a log with Go and infers it with Rust — proving the format contract
holds across both implementations.

> Transcripts below use bare `portcap` / `portsmith-replay` names. Substitute
> `bin/portcap` and `target/release/portsmith-replay`, or add them to `PATH`.

---

## Bench session (annotated transcript)

A complete loop on the bundled Redis sample — no Redis-specific code anywhere in
portsmith.

**1 &#183; Normalize a raw session log into a canonical trace.** The `redis.log`
uses `>` for client lines and `<` for server lines; consecutive same-direction
lines are coalesced into one record and timestamps are synthesized from
`-start` stepping by `-step`.

```console
$ portcap normalize -in samples/redis.log -proto redis -session a1b2c3 \
      -start 1700000000000000000 -step 50000000 -out samples/redis.trace
# normalized 6 records
```

**2 &#183; Read the trace back as escaped, human-readable text** (`cat` shows the
nanosecond timestamp, direction arrow, session, proto, and a control-escaped
preview of up to 64 payload bytes):

```console
$ portsmith-replay cat -in samples/redis.trace
 1700000000000000000 -> a1b2c3     redis  PING\n
 1700000000050000000 <- a1b2c3     redis  +PONG\n
 1700000000100000000 -> a1b2c3     redis  SET key1 hi\n
 1700000000150000000 <- a1b2c3     redis  +OK\n
 1700000000200000000 -> a1b2c3     redis  GET key1\n
 1700000000250000000 <- a1b2c3     redis  $2\nhi\n
```

**3 &#183; Chart the protocol's structure.** Inference groups records by
`(proto, direction)` and reports encoding, terminator, length statistics, and the
leading-token histogram (sorted by count, then name):

```console
$ portsmith-replay infer -in samples/redis.trace
portsmith schema inference
==========================

[redis request]
  samples    : 3
  encoding   : text
  terminator : LF
  length     : min=5 max=12 mean=8.7
  tokens     :
    GET              1
    PING             1
    SET              1

[redis response]
  samples    : 3
  encoding   : text
  terminator : LF
  length     : min=4 max=6 mean=5.3
  tokens     :
    $2               1
    +OK              1
    +PONG            1
```

**4 &#183; Summarize the recording** with `stats` (counts, byte totals, distinct
sessions, wall-clock span, protocol breakdown):

```console
$ portcap stats -in samples/redis.trace
records:   6
requests:  3
responses: 3
sessions:  1
bytes:     42
duration:  250ms
protocols:
  redis    6
```

**5 &#183; Replay the captured requests against a live server.** Only *request*
records are sent; each line shows request/response byte counts and a preview, and
a summary closes the run:

```console
$ portsmith-replay replay -in samples/redis.trace -target 127.0.0.1:6379
[a1b2c3] 5 bytes -> 5 bytes | +PONG\r\n
[a1b2c3] 12 bytes -> 5 bytes | +OK\r\n
[a1b2c3] 9 bytes -> 9 bytes | $2\r\nhi\r\n
replayed 3 request(s), 0 error(s), sent 26 byte(s), received 19 byte(s)
```

**6 &#183; Or clip the probe onto a live service** and record real traffic while a
client talks through the proxy:

```console
$ portcap capture -listen :9000 -target 127.0.0.1:6379 -proto redis -out cap.trace
# capturing :9000 -> 127.0.0.1:6379 (proto=redis) to cap.trace
# ...point a client at localhost:9000...
```

---

## The trace format, walked field by field

The trace is the single contract between the two tools. It is line-oriented
UTF-8 so both standard libraries can parse it trivially. The authoritative spec
lives in [`docs/FORMAT.md`](docs/FORMAT.md); here is the working reading of it.

A file is a recommended magic comment, any number of comments/blank lines, then
one record per line:

```
#portsmith-trace v1
V1 1700000000000000000 > a1b2c3 redis 5 UElORwo=
V1 1700000000050000000 < a1b2c3 redis 6 K1BPTkcK
```

Each record is seven space-separated fields. Because the payload is base64 it
contains no spaces, so a decoder splits on the **first six spaces** (`splitn(7)`):

```
V1   1700000000000000000   >   a1b2c3   redis   5   UElORwo=
│    │                     │   │        │       │   │
│    │                     │   │        │       │   └─ payload  base64(raw bytes) → "PING\n"
│    │                     │   │        │       └───── len      decimal length of the DECODED payload (5)
│    │                     │   │        └───────────── proto    protocol hint: tcp | http | redis | raw | …
│    │                     │   └────────────────────── session  opaque id, no spaces
│    │                     └────────────────────────── dir      ">" request (client→server) | "<" response
│    └──────────────────────────────────────────────── ts_nanos int64 nanoseconds since the Unix epoch
└───────────────────────────────────────────────────── version  literal "V1"
```

The `len` field is redundant with the payload **on purpose** — it lets a reader
detect truncation or corruption *before* allocating. Both decoders reject a
record when it does not split into exactly 7 fields, the version is not `V1`,
`ts_nanos` is not a valid int64, `dir` is neither `>` nor `<`, `len` is not a
valid non-negative integer, the base64 is invalid, or **the decoded length does
not match `len`**.

Other rules both implementations honor: line separator is `\n` (a trailing `\r`
is tolerated); blank/`#`-comment lines are ignored; the magic header is a comment
(recommended, not required); base64 is *standard* (`+`/`/`, `=` padding), with the
Rust side shipping its own known-answer-tested codec so it needs no crates; and
future revisions bump the version tag (`V2`, …) rather than being guessed at.

---

## Architecture

The `*.trace` file is the waist of the hourglass: Go writes it, Rust reads it,
and nothing else crosses the boundary.

```mermaid
flowchart LR
    client([Client]) -->|TCP| proxy
    subgraph GO["portcap · Go"]
        proxy[capture proxy] --> tw[trace.Writer]
        norm[normalize] --> tw
        stats[stats]
    end
    proxy -->|TCP| server([Upstream service])
    log[[raw > / < log]] --> norm
    tw -->|writes| trace[["*.trace<br/>portsmith v1"]]

    trace -->|reads| infer
    subgraph RS["portsmith-replay · Rust"]
        infer[schema infer] --> report[[schema report]]
        catcmd[cat]
        replaycmd[replay engine]
    end
    trace --> catcmd
    trace --> stats
    trace --> replaycmd
    replaycmd -->|TCP| target([Replay target])
```

*(Mermaid is used sparingly here; the animated SVGs below carry the visual load.)*

---

## Trace flow

The full pipeline as a workshop schematic — raw log tidied into base64 records,
records charted into a schema, requests driven back onto the wire:

<p align="center">
  <img src="docs/assets/trace-flow.svg" alt="portsmith trace flow: a raw redis.log is normalized into base64 trace records, inferred into a schema report, and replayed against a live target in a terminal" width="100%">
</p>

---

## Command reference

### `portcap` (Go)

| Command | Purpose | Key flags |
|---------|---------|-----------|
| `capture` | Transparent TCP proxy; records both directions to a trace. | `-listen` (default `:9000`), `-target` (**required**, `host:port`), `-proto` (default `tcp`), `-out` (default `-` = stdout), `-max` (stop after N connections; `0` = unlimited) |
| `normalize` | Convert a `> / <` raw session log into a canonical trace. | `-in` (default `-` = stdin), `-proto` (default `raw`), `-session` (default `norm0001`), `-out` (default `-`), `-start` (default: now, unix nanos), `-step` (default `1ms` in nanos) |
| `stats` | Summarize a trace: records, requests/responses, sessions, bytes, duration, protocol breakdown. | `-in` (default `-`) |
| `version` | Print `portcap 1.0.0`. | — |

### `portsmith-replay` (Rust)

| Command | Purpose | Key flags |
|---------|---------|-----------|
| `infer` | Heuristically discover the schema of each `(proto, direction)` group. | `-in` (default `-` = stdin) |
| `replay` | Send captured **requests** to a target; collect responses. | `-target` (**required**), `-timing` (preserve inter-request delays, capped at 2s), `-timeout` (per-read timeout ms, default `200`), `-wait` (max wait for a response ms, default `500`), `-in` (default `-`) |
| `cat` | Pretty-print a trace with control characters escaped. | `-in` (default `-`) |
| `version` | Print `portsmith-replay 1.0.0`. | — |

Run any command with `-h` for its flags. Both tools accept `-` (or an omitted
`-in`) to read from **stdin**, and Go's `-out`/Rust's stdout make them
pipe-friendly:

```sh
portcap normalize -in samples/redis.log -proto redis | portsmith-replay infer -in -
```

---

## Workflows

**A · From a hand-written log to a schema.** Fastest way to sketch a protocol you
can describe but not yet capture. Write a `> / <` log, normalize it, infer it.

```sh
portcap normalize -in session.log -proto myproto -out my.trace
portsmith-replay infer -in my.trace
```

**B · From a live service to a replay.** Proxy the real service, capture genuine
traffic, then replay it against a staging instance.

```sh
portcap capture -listen :9000 -target prod-box:6379 -proto redis -out cap.trace
# ...drive a client at localhost:9000, then Ctrl-C...
portcap stats  -in cap.trace                 # sanity-check what you recorded
portsmith-replay replay -in cap.trace -target staging-box:6379
```

**C · Timing-faithful regression.** Preserve the original inter-request gaps
(capped at 2 seconds per gap) to approximate the live cadence:

```sh
portsmith-replay replay -in cap.trace -target 127.0.0.1:6379 -timing -wait 1000
```

**D · Cross-language interop check** (the same one CI runs):

```sh
cd go && go run ./cmd/portcap normalize -in ../samples/redis.log \
    -proto redis -session a1b2c3 -start 1700000000000000000 -step 50000000 \
    -out ../samples/roundtrip.trace
cd ../rust && cargo run --quiet -- infer -in ../samples/roundtrip.trace
```

---

## Use cases

- **Reverse-engineering an undocumented daemon.** Capture a real session, run
  `infer`, and read off framing (LF/CRLF/none), text-vs-binary, and the
  verb/method vocabulary before writing any client code.
- **Regression &amp; smoke tests for line protocols.** Freeze a known-good session
  as a trace and replay it against each new build.
- **Load/soak stimulus.** Replay a representative trace to keep a service warm or
  exercise a code path repeatedly.
- **Documentation from evidence.** Turn a trace into a `cat` transcript and an
  `infer` report that describe what the protocol *actually* does.
- **Teaching &amp; demos.** The HTTP and Redis samples are a self-contained lesson
  in framing, base64, and request/response structure.

---

## How inference actually works

Inference is heuristic and dependency-free. Records are grouped by
`(proto, direction)`, and for each group `portsmith-replay` computes:

- **Encoding — text vs binary.** A payload is "printable" when at least 90% of its
  bytes are printable ASCII (plus tab/CR/LF); empty payloads count as printable. A
  group is **text** when at least 80% of its samples are printable, else **binary**.
- **Terminator — CRLF / LF / none.** Votes on how many payloads end in `\r\n`
  versus a bare `\n`: **CRLF** when CRLF is at least as common as LF and covers at
  least half the samples, **LF** when LF covers at least half, else **none**.
- **Length statistics.** `min`, `max`, and `mean` of decoded payload byte lengths.
- **Leading-token histogram.** The first whitespace-delimited token of each
  textual payload (e.g. HTTP methods, Redis verbs), capped at 32 bytes so binary
  noise never becomes a "token". Tokens are sorted by count descending, then name,
  and the top 8 are shown.

These are signals, not certainties — see [Limitations](#limitations).

---

## How replay actually works

`replay` groups records by session (preserving first-appearance order) and opens
**one TCP connection per session**. Within a session it walks records in order
and, for each **request**, writes the payload, flushes, then reads whatever the
server returns before moving on — tracking byte counts and a response preview.

The read window is bounded two ways: a per-read socket timeout (`-timeout`,
default 200 ms) and an overall wait per request (`-wait`, default 500 ms). Reads
stop early on a short read, EOF, or would-block/timeout. With `-timing`, the
replayer sleeps the original inter-record gap before each request, capped at 2 s
so a stale trace cannot stall the run.

Per-request failures (connect/write) are captured as errors in the report rather
than aborting the run; the process exits non-zero if any occurred. Response
records in the input are **not** sent — replay drives only the request side.

---

## Design choices

- **Why base64 payloads?** Payloads are arbitrary bytes — binary protocols,
  embedded NULs, non-UTF-8. Base64 keeps every record on a single ASCII-clean line
  that survives `grep`, `diff`, and copy-paste.
- **Why a redundant `len`?** A cheap integrity check and forward-compatible
  framing: a reader detects truncation before it allocates, and the decoder
  hard-fails on any mismatch.
- **Why nanoseconds?** High-resolution ordering and faithful timing replay.
- **Why two languages?** Capture is a concurrency/I/O problem (the proxy uses a
  goroutine per direction with a mutex-guarded writer); inference and replay are a
  parsing/state problem (Rust's exhaustive matching and ownership). Splitting them
  keeps each half small and idiomatic.
- **Why no dependencies?** Every line — including the base64 codec — is in-tree
  and testable, which is the whole point of a tool you use to understand *other*
  systems. Clippy runs `-D warnings`; the Go build is race-tested in CI.

---

## Comparison

Rough positioning — portsmith is intentionally narrow.

| Capability | **portsmith** | tcpdump / Wireshark | mitmproxy | Custom scripts |
|---|:--:|:--:|:--:|:--:|
| Capture live TCP traffic | ✅ app-level proxy | ✅ packet-level | ✅ HTTP(S) focus | ⚠️ you build it |
| Human-readable, greppable trace | ✅ base64 text | ⚠️ pcap (binary) | ⚠️ flows/pcap | ⚠️ varies |
| Protocol-agnostic (no dissectors) | ✅ | ⚠️ needs dissector | ❌ HTTP-centric | ⚠️ varies |
| Heuristic schema inference | ✅ text/binary, framing, tokens | ❌ | ❌ | ❌ |
| Replay requests to a live target | ✅ with optional timing | ❌ | ⚠️ limited | ⚠️ varies |
| Zero third-party dependencies | ✅ | ❌ | ❌ | ⚠️ varies |
| Scope | line-oriented req/resp | all packets | web traffic | anything |

If you need TLS interception, packet-level analysis, or rich dissectors, reach
for the specialized tools above. If you need to *understand and re-drive* a
line-oriented TCP protocol with something you can read end-to-end, that is
portsmith.

---

## Limitations

Being honest about the edges of the v1 toolchain:

- **Line/request-response oriented.** Normalization and inference assume text with
  framing discernible from terminators. Streaming, multiplexed, or length-prefixed
  binary protocols infer as `binary` with `terminator: none` and limited tokens.
- **Inference is heuristic.** The 90%/80% printable thresholds, terminator voting,
  and leading-token extraction describe a corpus; they do not prove a grammar.
- **Replay is stateless per request.** It writes a request and reads what returns
  within the wait window; it does not model correlation, sequence numbers, auth
  handshakes, or adaptive framing, and does not diff responses automatically.
- **`normalize` synthesizes timing** from `-start`/`-step`; only `capture` records
  real wall-clock nanos.
- **`capture` emits one record per TCP read** — a record boundary is a read
  boundary, not necessarily a protocol message boundary.
- **No TLS, no UDP.** Plain TCP only.

---

## Troubleshooting

| Symptom | Likely cause &amp; fix |
|---|---|
| `trace: line N: expected 7 fields, got M` | Line was hand-edited or truncated. Records are exactly 7 space-separated fields; the payload must be space-free base64. |
| `trace: line N: length mismatch: declared X, decoded Y` | `len` disagrees with the decoded payload — truncation or a bad edit. Re-generate the record; do not hand-tweak `len`. |
| `trace: line N: invalid base64 payload` | Payload is not *standard* base64 (`+`/`/`, `=` padding). URL-safe base64 is not accepted. |
| `trace: line N: unknown record version "V2"` | Trace is from a newer format revision; use a matching tool version. |
| `replay: -target is required` | Pass `-target HOST:PORT`. |
| `[sess] ERROR connect HOST:PORT: ...` | Target unreachable/refusing. The run continues and exits non-zero. |
| Replay responses look empty/truncated | Server was slower than the read window. Raise `-wait` and/or `-timeout`. |
| `capture` records nothing | No client traversed the proxy. Point your client at `-listen`, not the upstream. |
| `infer` shows `binary` for expected text | Payloads fall below the 90% printable threshold, or too few samples. |
| `cat`/`stats` print nothing | Empty trace or all comments/blanks. Confirm records start with `V1 `. |

---

## Repository layout

```
portsmith/
├── go/                  portcap (module github.com/portsmith/portcap)
│   ├── cmd/portcap/     CLI: capture · normalize · stats · version
│   └── internal/        trace codec (+tests) · TCP proxy · normalize+stats
├── rust/                portsmith-replay
│   ├── src/             trace.rs (codec + in-tree base64) · schema.rs · replay.rs · main.rs
│   └── tests/           integration tests (base64 KAT, parsing, inference)
├── samples/             redis.log, http.log + generated *.trace files
├── docs/
│   ├── FORMAT.md        authoritative trace format spec
│   └── assets/          animated SVG diagrams for this README
├── Makefile             build · test · fmt · vet · demo · clean
├── CHANGELOG.md
└── .github/workflows/   CI: Go · Rust · cross-language interop
```

---

## Roadmap

Directions under consideration (not yet implemented — see [Limitations](#limitations)
for what v1 does today):

- Response diffing in `replay` (compare live responses against recorded ones).
- Length-prefixed / delimiter-configurable framing hints for inference.
- Additional protocol hints and richer token analysis (e.g. header/verb pairs).
- A `V2` trace revision if framing metadata proves worth the redundancy.

Contributions that keep the "standard-library-only, readable end-to-end" spirit
are welcome.

---

## License

MIT — see [`LICENSE`](LICENSE).

<p align="center"><sub>portsmith · clip the probe, read the waveform, chart the schema, replay the signal.</sub></p>

<!-- keep this section short -->
