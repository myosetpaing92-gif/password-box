# Password Box: Phased Implementation Plan

> **For agentic workers:** Use the executing-plans skill to implement only the phase the user has requested. Track work with checkboxes. Stop after that phase for the user's review and questions; begin the next phase only when the user explicitly asks to continue.

**Date:** 2026-11-11

**Goal:** Build the Go CLI described in [DESIGN.md](DESIGN.md) in small phases that the user can review and understand.

**Architecture:** One local encrypted vault file stores all entry names, values, and metadata. The CLI coordinates separate packages for entry rules, cryptography, filesystem storage, hidden terminal input, and the Linux clipboard.

**Tech stack:** Go, Cobra, `golang.org/x/crypto`, `golang.org/x/term`, `golang.org/x/sys/unix`, and the Go standard library. Pin dependency versions when they are introduced.

## How We Will Work Together

1. The user requests a phase, for example: **"Start Phase 1."**
2. Before coding, explain the goal, the files involved, and the concepts needed for that phase. Break the phase into small tasks with a focused test and implementation cycle.
3. Implement only that phase. For behavior that needs tests, first demonstrate the relevant failure, then implement and verify the behavior.
4. Show what changed, why it changed, the checks actually run, and a small example using dummy data.
5. Mark the phase **Awaiting review** and stop.
6. The user reviews the changes and asks questions. Answer with examples and explain unfamiliar code; questions do not count as permission to advance.
7. Make corrections within the current phase if requested. Record approval when the user approves it.
8. Start the next phase only after an explicit request to continue.

The current task creates this roadmap. Implementation begins with a separate request to start Phase 1.

### Phase Statuses

- **Not started:** No implementation work has begun.
- **In progress:** The current phase is being implemented.
- **Awaiting review:** The work and verification evidence are ready for the user.
- **Approved:** The user has reviewed and accepted the phase.

Passing tests does not automatically mark a phase approved.

## Global Constraints

Every phase follows [DESIGN.md](DESIGN.md). These constraints apply throughout implementation:

- Linux first; macOS and Windows are later releases.
- One encrypted vault file; no partitioning or server account.
- Explicit consent before creating storage; refusal creates no application files.
- Entry names, values, and entry timestamps are inside the encrypted payload.
- Master password creation/change: at least 15 characters and at most 1,024 UTF-8 bytes; preserve the exact input.
- Never accept the master password through arguments or environment variables.
- Prompt for the master password on every command that accesses vault contents; no persistent unlock session.
- Argon2id: 65,536 KiB memory, 3 iterations, parallelism 4, 32-byte output, 16-byte random salt.
- XChaCha20-Poly1305: 24-byte fresh random nonce per encryption; authenticate the exact header bytes.
- Payload limit: 16 MiB. Header: 59 bytes. Authentication tag: 16 bytes. Maximum envelope: 16 MiB plus 75 bytes.
- Keys: case-sensitive, unique, 1–128 ASCII characters, first character alphanumeric; remaining characters are letters, digits, `.`, `_`, `/`, or `-`.
- Values: valid UTF-8 text, 1–65,536 bytes, no NUL; preserve whitespace and newlines.
- Private directory: `0700`. Vault, lock, backups, and encrypted temporary files: `0600`.
- No plaintext temporary files, secret logs, or secret-value arguments.
- Acquire locks before opening the authoritative vault; preserve the stable lock file.
- Mutations use encrypted temporary files, atomic replacement, file sync, and directory sync.
- Initialization and backup creation never overwrite an existing destination.
- Clipboard expiry is best effort; cleanup must not overwrite newer clipboard content.
- Use dummy secrets and a disposable private directory for demonstrations.
- Keep cloud sync, sharing, password recovery, key renaming, and background unlock sessions outside the first release.

## Phase Overview

Phases run in order. Each phase builds on the approved work before it.

| Phase | Deliverable | What you will learn | Status |
| --- | --- | --- | --- |
| 1 | Runnable CLI foundation | Go modules, packages, commands, exit codes | Not started |
| 2 | Entry rules in memory | Structs, validation, errors, unit tests | Not started |
| 3 | Password-based encryption | Key derivation, salts, nonces, authentication | Not started |
| 4 | Versioned vault format | Binary headers, serialization, bounded parsing | Not started |
| 5 | Private filesystem access | Linux paths, ownership, permissions, file descriptors | Not started |
| 6 | Safe saves and locking | Concurrent processes, atomic replacement, durability | Not started |
| 7 | Consent and vault initialization | Hidden input, orchestration, integration tests | Not started |
| 8 | List and show commands | Read operations, stdout/stderr, safe output | Not started |
| 9 | Add, edit, and delete commands | Read-modify-write transactions and confirmations | Not started |
| 10 | Master password changes | Re-encryption, salt replacement, old-copy limits | Not started |
| 11 | Encrypted backup and restore | Snapshots, validation, restoration safety | Not started |
| 12 | Clipboard support | Child processes, cancellation, platform adapters | Not started |
| 13 | Hardening and failure testing | Fuzzing, race checks, failure injection, benchmarks | Not started |
| 14 | Linux release and usage guide | Building, checksums, installation, release checks | Not started |

## File Responsibilities

Create files when their phase needs them. The following map identifies the main boundaries; corresponding `_test.go` files live beside the production files they verify.

| Path | Responsibility | First phase |
| --- | --- | --- |
| `go.mod`, `go.sum` | Module identity and pinned dependencies | 1 |
| `cmd/pbox/main.go` | Start the command and return its exit status | 1 |
| `internal/cli/root.go`, `version.go`, `errors.go` | Command setup, version output, error presentation | 1 |
| `internal/vault/model.go`, `validation.go`, `operations.go`, `errors.go` | In-memory entries and business rules | 2 |
| `internal/cryptobox/kdf.go`, `aead.go` | Password-derived keys and authenticated encryption | 3 |
| `internal/storage/envelope.go` | Encode and parse the public header and ciphertext | 4 |
| `internal/vault/codec.go` | Serialize and validate the decrypted JSON payload | 4 |
| `internal/storage/paths_linux.go`, `permissions_linux.go` | Resolve paths and validate private filesystem objects | 5 |
| `internal/storage/lock_linux.go`, `atomic_linux.go` | Stable locks and durable encrypted writes | 6 |
| `internal/terminal/input_linux.go` | Consent prompts and hidden controlling-terminal input | 7 |
| `internal/vault/service.go` | Coordinate unlock, validation, and storage transactions | 7 |
| `internal/cli/init.go` | Initialize a vault after consent | 7 |
| `internal/cli/list.go`, `show.go`, `output.go` | List keys and deliberately reveal values | 8 |
| `internal/cli/add.go`, `edit.go`, `delete.go` | Entry mutation commands | 9 |
| `internal/cli/passwd.go` | Change the master password | 10 |
| `internal/cli/backup.go`, `restore.go` | Snapshot and restore commands | 11 |
| `internal/clipboard/clipboard_linux.go`, `wayland_linux.go`, `x11_linux.go` | Desktop clipboard backends | 12 |
| `internal/cli/copy.go` | Copy an unlocked value | 12 |
| `README.md`, `docs/usage.md`, `docs/security.md` | Installation, command examples, and security limits | 14 |

Avoid creating empty packages or registering unimplemented commands. Add a command when its phase makes it functional.

## Phase 1: Go Project and CLI Foundation

**Goal:** Produce an executable that prints help and version information.

**Work:**

- [ ] Inspect the installed Go toolchain and choose a compatible module setup. Infer the module path from the repository identity when available.
- [ ] Create the entry point and command package; add and pin Cobra.
- [ ] Implement `pbox --help`, `pbox version`, and clear handling of unknown commands.
- [ ] Establish stdout/stderr separation and the design's exit-code mapping for implemented errors.
- [ ] Keep application logic outside `main.go` so it can be exercised without exiting the test process.

**Checks:** Command tests cover help, version, and invalid usage. Run `go test ./internal/cli`, `go build ./cmd/pbox`, and the following demonstration:

```bash
go run ./cmd/pbox --help
go run ./cmd/pbox version
```

**Learn and review:** Explain `go.mod`, `go.sum`, `package main`, imports, `cmd/`, `internal/`, Cobra commands, and process exit codes. Show how one command moves from the entry point to its handler.

**Ready for review when:** The CLI builds and its help/version commands work. No vault storage is created by these commands.

## Phase 2: Entry Model and Validation

**Goal:** Manage entries in memory without involving encryption or the filesystem.

**Work:**

- [ ] Define the vault document, entry, schema version, revision, and UTC timestamp fields.
- [ ] Implement the exact key and value limits from the design.
- [ ] Implement add, find, list, replace-value, and delete operations with typed errors.
- [ ] Sort listed keys and preserve creation timestamps during edits.
- [ ] Keep each successful document mutation consistent with the revision rule; prevent revision overflow.

**Checks:** Run `go test ./internal/vault`. Cover duplicate keys, missing entries, key boundaries, invalid UTF-8, NUL, multibyte value byte limits, whitespace preservation, sorted results, timestamps, and unchanged state after rejected mutations.

**Learn and review:** Explain structs, slices, pointer receivers, returned errors, table-driven tests, and how tests use a fixed time to avoid flaky expectations. Walk through an add/edit/delete lifecycle using a test entry named `demo/token`.

**Ready for review when:** Entry behavior is independently tested. This phase stores no plaintext on disk.

## Phase 3: Password-Based Encryption

**Goal:** Encrypt and authenticate byte slices in memory using established Go libraries.

**Work:**

- [ ] Add the pinned crypto dependency and implement the fixed Argon2id profile.
- [ ] Implement authenticated encryption/decryption with caller-supplied associated data.
- [ ] Generate salts and nonces using `crypto/rand`; reject invalid lengths and propagate randomness failures.
- [ ] Normalize authentication failures without claiming to distinguish a wrong password from tampering.
- [ ] Limit key/plaintext lifetime and clear owned byte buffers where practical.

**Checks:** Run `go test ./internal/cryptobox`. Cover a successful round trip, wrong password, changed ciphertext, changed associated data, invalid key/nonce lengths, and fresh nonces. Verify repeated encryption does not produce identical envelopes in the later format integration.

**Learn and review:** Explain why a password needs a key derivation function, why the salt is public, why every encryption needs a fresh nonce, and how authentication detects tampering. Distinguish hashing from reversible encryption.

**Ready for review when:** Correct inputs decrypt and authentication failures return errors without exposing plaintext.

## Phase 4: Vault Envelope and Payload Encoding

**Goal:** Turn the design's file format into a bounded, versioned encoding.

**Work:**

- [ ] Encode and parse the 59-byte big-endian header with the `PBXVAULT` identifier.
- [ ] Define version one's algorithms, Argon2 version `0x13`, and exact accepted KDF settings.
- [ ] Enforce the maximum envelope size and minimum ciphertext length before expensive work.
- [ ] Serialize the document as JSON and reject duplicate entries, unsupported schemas, malformed timestamps, duplicate object fields, and extra trailing JSON data.
- [ ] Connect header bytes to authenticated encryption as associated data; preserve the exact bytes read for verification.

**Checks:** Run `go test ./internal/storage ./internal/vault ./internal/cryptobox`. Cover known header bytes, bad magic, unsupported versions, truncated files, oversized input, changed headers, invalid KDF settings, invalid documents, and a complete encode/encrypt/decrypt/decode round trip.

**Learn and review:** Explain a binary file layout, endianness, public headers versus encrypted contents, JSON serialization, and why parsing happens in stages.

**Ready for review when:** The complete format works in memory and malformed inputs fail before unbounded allocation or key derivation.

## Phase 5: Private Linux Storage Locations

**Goal:** Access only validated private filesystem objects at the selected vault location.

**Work:**

- [ ] Resolve `$XDG_DATA_HOME/password-box/vault.pbox`, the home-directory fallback, and explicit `--vault` paths without creating anything during resolution.
- [ ] Ignore relative XDG paths and validate custom private directories.
- [ ] Create directories with `0700` and files with `0600` when explicitly requested by the caller.
- [ ] Validate ownership, object type, permissions, symlinks, and unexpected hard links.
- [ ] Anchor subsequent operations to an opened, validated directory rather than repeatedly trusting pathname checks.

**Checks:** Run `go test ./internal/storage`. Use disposable directories to test default/custom paths, relative XDG values, unsafe permissions, wrong object types, symlinks, and hard links. Confirm path lookup itself creates nothing.

**Learn and review:** Explain absolute paths, Linux user ownership, permission bits, `0700` versus `0600`, file descriptors, and the race between checking a pathname and opening it.

**Ready for review when:** Safe locations work and unsafe objects fail clearly without silently broadening permissions or changing unrelated files.

## Phase 6: Durable Saves and Concurrent Access

**Goal:** Preserve a valid encrypted vault during simultaneous commands and interrupted writes.

**Work:**

- [ ] Implement shared/exclusive advisory locks on the stable companion lock file, with cancellation and a bounded wait.
- [ ] Ensure lock descriptors are closed on execution of child processes and never unlink the lock file during normal operation.
- [ ] Implement no-replacement publication for initial creation and encrypted backup destinations.
- [ ] Implement same-directory encrypted temporary writes, file sync, atomic replacement, and directory sync.
- [ ] Inject failures at write, sync, close, and publication boundaries; distinguish a failure before replacement from uncertain durability afterward.
- [ ] Exercise real competing subprocesses to verify that locks protect the entire read-modify-write operation.

**Checks:** Run `go test ./internal/storage`. Demonstrate that two competing writers serialize, the lock becomes available after its owning process exits, cancellation stops waiting, and failed pre-replacement writes preserve the old bytes.

**Learn and review:** Explain why atomic replacement and durability are different, why the directory needs syncing, why locking the replaced vault inode is unsafe, and why a leftover lock file is normal.

**Ready for review when:** Safe storage primitives pass concurrency and failure tests. The tests use dummy ciphertext bytes, not real secrets.

## Phase 7: Consent, Hidden Input, and Initialization

**Goal:** Create a real encrypted empty vault through `pbox init`.

**Work:**

- [ ] Add the pinned terminal dependency and hidden master-password input from the controlling terminal.
- [ ] Show the destination, growing-file behavior, password requirement, and lack of password recovery before asking for consent.
- [ ] Default consent to no; make refused consent and cancelled password input leave no application files.
- [ ] Validate password length, confirmation, and exact-byte preservation before creating storage.
- [ ] Connect the model, cryptography, format, and storage through the vault service; create revision 1 without replacing existing data.
- [ ] Restore terminal state on handled cancellation and report noninteractive failures clearly.

**Checks:** Run `go test ./internal/terminal ./internal/cli ./internal/vault ./internal/storage`. Cover refusal, mismatched passwords, length limits, missing terminal, existing destinations, and successful creation. Manually run `pbox init` in a disposable private directory and inspect the resulting permissions.

**Learn and review:** Explain how a service coordinates smaller packages, why prompts use stderr, how hidden input works, and how integration tests differ from isolated unit tests.

**Ready for review when:** Consent creates an unlockable empty encrypted vault; refusal creates nothing; reinitialization preserves existing bytes.

## Phase 8: List and Show Entries

**Goal:** Read an existing vault through authenticated CLI commands.

**Work:**

- [ ] Add the shared-lock/read/unlock/validate flow and bounded reads.
- [ ] Implement `list` with sorted keys and no values.
- [ ] Implement `show <key>` with escaped terminal control characters and a terminal requirement for ordinary output.
- [ ] Implement explicit `show --raw <key>` with exact value bytes and no added newline.
- [ ] Map authentication, missing-entry, malformed-vault, and I/O failures to the design's exit codes.

**Checks:** Run `go test ./internal/cli ./internal/vault`. Use an encrypted test fixture built through the earlier packages. Verify wrong-password rejection, key-only listing, missing entries, safe escaping, exact raw bytes, and refusal of accidental redirected ordinary output.

**Learn and review:** Explain a read-only command's data flow, why listing still needs a password, how pipes work, and the difference between safe display and deliberate raw output.

**Ready for review when:** Reading the fixture works and does not mutate the vault. The real CLI can list the empty vault from Phase 7.

## Phase 9: Add, Edit, and Delete Entries

**Goal:** Complete the core key-value workflow with durable transactions.

**Work:**

- [ ] Implement `add` and `edit` with hidden value prompts; expose no secret-value argument.
- [ ] Support `--stdin` for multiline values while reading the master password from `/dev/tty`.
- [ ] Implement confirmed deletion and explicit `--yes`.
- [ ] Keep the exclusive lock through read, unlock, mutation, serialization, encryption, and durable save.
- [ ] Enforce the total serialized payload limit before publication; leave the original unchanged on rejected input or mutation.

**Checks:** Run `go test ./internal/cli ./internal/vault ./internal/storage`. Cover duplicates, missing entries, declined deletion, multiline input, payload limits, revision/timestamps, concurrent CLI mutations, and absence of secret values in success/errors.

**Demonstration:** Add a dummy `demo/token`, list it, display it, edit it, and delete it. Type the dummy value only into the hidden prompt.

**Learn and review:** Explain the complete read-modify-write transaction and identify where validation, authentication, locking, and persistence happen.

**Ready for review when:** The requested add/list/view/edit/delete workflow works end to end, including concurrent writes and invalid input.

## Phase 10: Master Password Changes

**Goal:** Change the password without losing entries.

**Work:**

- [ ] Add `pbox passwd` with current-password authentication and new-password confirmation.
- [ ] Reuse creation-time password validation.
- [ ] Generate a new salt and nonce; derive the new key and re-encrypt under the exclusive lock.
- [ ] Preserve entries and their timestamps, increment the revision, and save through the existing transaction.

**Checks:** Run `go test ./internal/cli ./internal/vault`. Verify new-password success, old-password failure on the updated vault, unchanged entries, rejected mismatches, wrong-current-password behavior, and preservation of the original on pre-replacement failure.

**Learn and review:** Explain why changing a password requires re-encryption and why old backups can still be opened with their original passwords.

**Ready for review when:** The new password opens the current vault and entries remain intact.

## Phase 11: Encrypted Backup and Restore

**Goal:** Recover entries from an authenticated encrypted snapshot.

**Work:**

- [ ] Implement `backup` from a validated snapshot with private permissions, no-overwrite publication, and durable completion.
- [ ] Implement bounded reading and full authentication/validation of a backup before restoration.
- [ ] For an existing destination, authenticate its current password, confirm replacement, preserve restored entry timestamps, and increment the destination revision.
- [ ] Re-encrypt restored data using the destination password and a fresh nonce.
- [ ] For a new destination, obtain creation consent, request a new password, and initialize a new document revision.

**Checks:** Run `go test ./internal/cli ./internal/vault ./internal/storage`. Cover backup overwrite refusal, corrupted backup, wrong passwords, declined restore, existing/new destinations, and restore after a password change. Verify that backup creation never writes plaintext.

**Learn and review:** Explain snapshots, source versus destination passwords, backup validation, and why restoring old data is separate from preventing malicious rollback.

**Ready for review when:** A backup restores the expected entries safely without silently replacing a destination or bypassing authentication.

## Phase 12: Linux Clipboard Support

**Goal:** Copy an entry without displaying its value or exposing it in process arguments.

**Work:**

- [ ] Define the clipboard boundary and implement Wayland `wl-copy` and X11 `xclip` adapters.
- [ ] Choose the matching desktop backend and report missing dependencies or a headless session clearly.
- [ ] Pass the selected value through child stdin without a shell; release vault locks and discard other decrypted data before waiting.
- [ ] Keep the clipboard owner in the foreground for up to 30 seconds; terminate that owner on expiry or cancellation.
- [ ] Avoid overwriting newer clipboard content and use the sensitive-content hint when the backend supports it.

**Checks:** Run `go test ./internal/clipboard ./internal/cli`. Test child arguments/stdin, timeout, cancellation, cleanup, and backend errors using controlled fake processes. Perform real desktop tests on both Wayland and X11 and record which sessions were actually verified.

**Learn and review:** Explain child processes, stdin, contexts, desktop clipboard ownership, and why clipboard history makes expiry a best-effort feature.

**Ready for review when:** Copy works on verified backends, cleanup leaves newer clipboard selections untouched, and unsupported environments produce useful errors. Keep any unverified backend limitation explicit.

## Phase 13: Security and Reliability Verification

**Goal:** Test the assembled application against malformed input, failures, and realistic limits.

**Work:**

- [ ] Add bounded fuzz targets for envelope parsing and decrypted-payload decoding. Avoid running Argon2 on every fuzz input.
- [ ] Revisit all required checks in DESIGN.md and add any missing boundary, subprocess, or cancellation cases.
- [ ] Run race checks and injected write/sync failures; inspect crash leftovers to ensure they contain only encrypted bytes.
- [ ] Inspect application-created files for deliberately distinctive dummy secrets and entry names.
- [ ] Benchmark unlock time and peak memory with small and maximum-size payloads; report measurements rather than promising fixed timing.
- [ ] Run static checks and the Go vulnerability scanner; investigate findings and record any unresolved limitation.

**Checks:** Run the commands below once the fuzz targets have been introduced, plus the targeted failure/concurrency tests:

```bash
go test ./...
go test -race ./...
go vet ./...
go test ./internal/storage -run '^$' -fuzz '^FuzzParseEnvelope$' -fuzztime=30s
go test ./internal/vault -run '^$' -fuzz '^FuzzDecodeDocument$' -fuzztime=30s
govulncheck ./...
```

**Learn and review:** Explain the difference between example-based tests and fuzzing, a data race versus a lost update, and what automated checks can and cannot prove about security.

**Ready for review when:** Results and remaining limitations are documented with actual evidence. A test suite is not presented as proof that all attacks are impossible.

## Phase 14: Linux Packaging and User Guide

**Goal:** Produce a Linux binary and documentation that a new user can follow.

**Work:**

- [ ] Build a versioned Linux binary and record the tested Go/dependency versions and supported architecture.
- [ ] Generate a SHA-256 checksum for the release artifact.
- [ ] Write installation/uninstallation instructions, CLI examples, storage locations, clipboard dependencies, and troubleshooting guidance.
- [ ] Document backups, password changes, lack of password recovery, plaintext-output choices, and SSD/clipboard limits.
- [ ] Perform the documented workflow with a fresh home/data directory and only dummy secrets.
- [ ] Re-run checks appropriate to packaging changes and record the final release evidence.

**Checks:** Build using `go build -o dist/pbox ./cmd/pbox`, run `./dist/pbox version` and `./dist/pbox --help`, and execute the documented init/add/list/show/edit/copy/delete/password/backup/restore lifecycle in a disposable private directory. Only claim platform/desktop support that was exercised.

**Learn and review:** Explain source code versus a binary, version information, checksums, installation paths, and the difference between compilation and platform verification.

**Ready for review when:** A fresh Linux user can follow the guide and reproduce the supported workflow. Release publication is a separate explicit action.

## Design Coverage

The 12 broad steps in DESIGN.md are expanded here into smaller review checkpoints. This changes the pacing, not the product scope.

| DESIGN.md requirement | Covered by phases |
| --- | --- |
| Consent and initialization | 5, 6, 7 |
| Threat model and security limitations | 3, 6, 10, 11, 12, 13, 14 |
| Password handling and encryption | 3, 4, 7, 10 |
| Versioned file format and entry schema | 2, 4 |
| CLI commands, input/output, and exit codes | 1, 7, 8, 9, 10, 11, 12 |
| Private files, locking, and durable writes | 5, 6, 7, 9, 11 |
| Clipboard behavior | 12 |
| Password changes, backup, and restore | 10, 11 |
| Package boundaries | 1–12, according to the file map |
| Failure tests, fuzzing, and performance measurements | Relevant phase checks plus 13 |
| Linux packaging and documented limitations | 14 |
| Later operating systems and optional features | Outside these phases, as specified in DESIGN.md |

## Review Handoff for Every Phase

At the end of a phase, provide this short report:

```text
Phase number and name:
Status: Awaiting review

What now works:
Files changed and their responsibilities:
Main ideas explained in plain language:
Checks actually run and their results:
A small demonstration you can try:
Known limitations at this point:

Waiting for your review and questions.
```

Record review notes under the current phase as they arise. When the user approves, update the status table. Preserve the explicit stop between phases even if later work looks straightforward.
