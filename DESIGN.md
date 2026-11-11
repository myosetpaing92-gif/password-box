# Password Box: Design and Build Plan

- Date: 2026-11-09
- Document status: Proposed design for review
- Implementation language: Go
- Initial platform: Linux; macOS and Windows deferred

## Overview

Password Box is a local encrypted key-value vault with a command-line interface named `pbox`.

The user approves the creation and location of one encrypted vault file, sets a master password, and manages entries through the CLI. The file grows as entries are added. It requires ordinary filesystem access under the user's account; it does not reserve a disk partition or require administrator privileges.

The confirmed scope is an encrypted vault file with Linux support first.

## 1. Scope of the First Release

The first version provides:

- Explicit consent before creating a vault.
- A master password that protects the vault's contents.
- Commands to add, list, view, edit, delete, and copy entries.
- Password changes.
- Encrypted backups and restoration.
- Safe handling of interrupted writes and concurrent commands.

Cloud synchronization, sharing, browser integration, password recovery, and background unlock sessions are deferred.

Authentication happens locally: knowing the master password allows the application to decrypt the vault. There is no server or separate user account.

## 2. Storage Approach

There are three reasonable approaches within the encrypted-file model:

| Approach | Benefits | Costs |
| --- | --- | --- |
| Encrypt the entire vault | Hides names, values, and metadata; straightforward backup | Reads and rewrites the entire payload |
| Embedded database with encrypted records | Efficient individual updates | Indexes and metadata need separate protection; more complex migrations |
| Encrypted SQLite | Transactions and queries | Adds encryption integration and packaging work |

Recommendation: encrypt the entire vault.

For a personal store of passwords, tokens, and configuration secrets, the expected data volume is small. Whole-vault encryption gives the first version a manageable design.

Set an initial plaintext payload limit of **16 MiB**. Reconsider the storage architecture if real usage outgrows that limit.

## 3. User Consent and Initialization

Running:

```bash
pbox init
```

shows a consent prompt similar to:

```text
Password Box will create an encrypted vault at:

  /home/alice/.local/share/password-box/vault.pbox

The file grows as you add entries.
Entry names and values will be encrypted.
You will need your master password to open it.
If you forget your password, this version cannot recover your data.

Create this vault? [y/N]
```

Only an explicit affirmative response permits creation. Refusal leaves no application files behind.

After consent:

1. Ask for a master password without echoing it.
2. Ask for confirmation.
3. Validate the password.
4. Create a private application directory.
5. Generate the cryptographic salt and encryption nonce.
6. Encrypt an empty vault.
7. Save it durably.
8. Report the created location.

Proposed password policy:

- At least 15 characters when creating or changing a password.
- Maximum 1,024 UTF-8 bytes.
- Recommend a long, unique passphrase.
- Preserve the password exactly; do not trim or normalize it.

Initialization must refuse to overwrite an existing vault.

A custom location is available through:

```bash
pbox --vault /path/to/private-directory/vault.pbox init
```

Use the Linux data location `$XDG_DATA_HOME/password-box`, falling back to `~/.local/share/password-box`. Ignore relative XDG paths, following the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/).

## 4. Security Model

### Intended Protections

The design protects against:

- Someone reading a copied vault file without its password.
- Accidental exposure of plaintext through application storage.
- Undetected modification of authenticated vault contents.
- Lost updates from simultaneous CLI commands.
- Partial-file corruption caused by interrupted saves.

### Practical Limits

The design cannot guarantee protection against:

- Malware or an administrator observing an unlocked process.
- A keylogger capturing the master password.
- Weak passwords being guessed offline.
- Terminal recording after a user explicitly displays a secret.
- Clipboard history retaining a copied value.
- Deletion or replacement of the entire vault by someone with filesystem access.

An older, valid vault can also be substituted for the current one. A local revision counter helps diagnose changes but does not prevent rollback without an independently trusted reference.

Deleting an entry removes it from the current vault. It does not guarantee erasure from SSD blocks, filesystem snapshots, or backups.

## 5. Encryption and Password Handling

### Cryptographic Choices

| Purpose | Proposed choice |
| --- | --- |
| Password-based key derivation | Argon2id |
| Derived key | 32 bytes |
| Salt | 16 random bytes |
| Authenticated encryption | XChaCha20-Poly1305 |
| Nonce | 24 fresh random bytes per encryption |
| Randomness | Go's `crypto/rand` |

Use `argon2.IDKey` with an initial fixed profile:

```text
Memory:       65,536 KiB — 64 MiB
Iterations:   3
Parallelism:  4
Output:       32 bytes
```

This matches RFC 9106's second recommended profile and the corresponding guidance in [Go's Argon2 documentation](https://pkg.go.dev/golang.org/x/crypto/argon2).

Use `chacha20poly1305.NewX` for authenticated encryption. Its key and nonce sizes are documented in [Go's XChaCha20-Poly1305 package](https://pkg.go.dev/golang.org/x/crypto/chacha20poly1305).

### Unlock Sequence

```text
Master password + stored salt
              |
              v
           Argon2id
              |
              v
       32-byte encryption key
              |
              v
   Authenticate and decrypt payload
              |
              v
       Parse and validate entries
```

A separate password hash is unnecessary: successful authenticated decryption validates the derived key.

If authentication fails, report:

```text
Could not unlock vault: incorrect password or damaged vault.
```

Do not claim to distinguish those two cases.

### Operational Rules

- Never store the master password.
- Never accept it through command arguments or environment variables.
- Prompt once per command that accesses vault contents.
- Keep keys and plaintext in memory only as long as needed.
- Avoid unnecessary conversions of secrets into immutable Go strings.
- Clear owned byte buffers where practical, while documenting that Go does not guarantee complete memory erasure.
- Never write plaintext temporary files.
- Never log passwords, values, encryption keys, or decrypted payloads.

For this small vault, derive the encryption key directly from the password. A separate wrapped data key adds complexity without a strong first-release need.

Keep the existing salt during ordinary mutations and generate a fresh nonce for every encryption. Generate a new salt when changing the password.

## 6. Vault File Format

Use a versioned binary envelope around an encrypted JSON payload:

```text
Fixed header
Encrypted payload with authentication tag
```

Proposed version-one header:

| Field | Size |
| --- | --- |
| Magic identifier: `PBXVAULT` | 8 bytes |
| Format version | 2 bytes |
| Argon2 memory setting | 4 bytes |
| Argon2 iteration setting | 4 bytes |
| Argon2 parallelism | 1 byte |
| Salt | 16 bytes |
| Nonce | 24 bytes |

The fixed header is 59 bytes. The authentication tag adds 16 bytes, so the maximum file size is the 16 MiB payload limit plus 75 bytes.

Use big-endian encoding for multibyte header integers. Algorithms and the Argon2 version are defined by the file-format version. Format version one uses Argon2id version 0x13 and XChaCha20-Poly1305.

Authenticate the exact header bytes as additional authenticated data. This binds the header to the ciphertext.

The header is public. It exposes format and cryptographic parameters, but no entry names or values. File size and filesystem timestamps remain observable.

### Parsing Rules

Before running Argon2 or allocating a large buffer:

- Check the magic identifier and supported version.
- Enforce the maximum file size.
- Check minimum ciphertext length.
- Validate header fields.
- Accept only the defined version-one KDF profile.

Do not pass attacker-controlled, unbounded memory or iteration values into Argon2.

A later cryptographic profile should be introduced through an explicit format upgrade.

## 7. Entry Model

The decrypted payload contains:

```json
{
  "schema_version": 1,
  "revision": 1,
  "entries": [
    {
      "key": "github/token",
      "value": "example-secret",
      "created_at": "2026-11-09T12:00:00Z",
      "updated_at": "2026-11-09T12:00:00Z"
    }
  ]
}
```

This is an illustration of the in-memory representation. The application stores its encrypted form.

Proposed rules:

| Field | Rule |
| --- | --- |
| Key | Unique and case-sensitive |
| Key length | 1–128 ASCII characters |
| Key characters | Letters, digits, `.`, `_`, `/`, `-`; first character alphanumeric |
| Value | UTF-8 text, 1–65,536 bytes; reject NUL |
| Timestamps | UTC |
| Payload size | At most 16 MiB |

A key such as `github/token` is a label; it never becomes a filesystem path.

Preserve value whitespace and newlines. Reject duplicate keys, unsupported schemas, malformed timestamps, and invalid entries after decryption.

The initial empty vault has revision 1. Each successful mutation increments the revision. Editing a value preserves its creation timestamp and updates its modification timestamp.

## 8. CLI Behavior

| Command | Behavior |
| --- | --- |
| `pbox init` | Obtain consent and create a vault |
| `pbox add <key>` | Prompt for a hidden value and create an entry |
| `pbox list` | Show sorted keys without values |
| `pbox show <key>` | Display one value |
| `pbox edit <key>` | Replace an existing value |
| `pbox delete <key>` | Confirm and remove an entry |
| `pbox copy <key>` | Copy one value to the desktop clipboard |
| `pbox passwd` | Change the master password |
| `pbox backup <destination>` | Create an encrypted snapshot |
| `pbox restore <backup>` | Validate and restore a snapshot |
| `pbox version` | Show application and format versions |

Examples:

```bash
pbox add github/token
pbox list
pbox show github/token
pbox edit github/token
pbox copy github/token
pbox delete github/token
```

### Input and Output Rules

- `add` refuses an existing key.
- `edit` refuses a missing key and replaces only the value; key renaming is deferred.
- `delete` requires confirmation; `--yes` explicitly skips that confirmation.
- Interactive values are entered without echo.
- No secret-value argument such as `--value secret`.
- No external editor in the first release, avoiding plaintext editor files.
- Prompts, progress, and errors go to stderr.
- Requested data goes to stdout.
- Human-readable output escapes terminal control characters.

For multiline values, support:

```bash
pbox add service/config --stdin
```

Read the value from stdin and the master password from `/dev/tty`. Apply the same input option to `edit`. Fail clearly if no controlling terminal exists.

Ordinary `show` requires stdout to be a terminal. An explicit `show --raw` permits exact output to a pipe or redirected file, making plaintext output a deliberate choice. Raw output contains only the value bytes, without an added newline.

There is no persistent `unlock` command in version one. Each content-access command unlocks its own process.

## 9. File Safety and Concurrency

Use:

```text
password-box/
  vault.pbox
  vault.pbox.lock
```

Proposed permissions:

- Application directory: `0700`.
- Vault, lock, backups, and encrypted temporary files: `0600`.

Validate ownership and permissions. Reject unsafe vault or lock objects, including symlinks and unexpected hard links. Anchor operations to a validated directory descriptor so checks are not merely vulnerable pathname checks.

Custom locations must use a private directory with equivalent ownership and permissions.

Support local filesystems initially. Network filesystems and simultaneous access through cloud synchronization tools are outside the supported first-release configuration.

### Locking

Use a stable companion lock file with Linux advisory locking:

- Shared lock for reading a snapshot.
- Exclusive lock for mutations.
- Acquire the lock before opening the authoritative vault.
- Keep the exclusive lock through durable replacement.
- Use a bounded wait and return a clear "vault busy" error.
- Never unlink the lock file during normal operation.
- Prevent clipboard child processes from inheriting lock descriptors.

Linux releases these locks when the relevant descriptors close; the mere presence of the lock file does not mean the vault is locked. See [Linux `flock`](https://man7.org/linux/man-pages/man2/flock.2.html).

### Saving Changes

Every mutation follows this sequence:

1. Acquire the exclusive lock.
2. Read, unlock, and validate the current vault.
3. Apply the requested change.
4. Validate limits and increment the revision.
5. Generate a fresh nonce and encrypt the complete payload.
6. Create an exclusive temporary file in the same directory.
7. Write the encrypted envelope.
8. Sync and close the temporary file.
9. Atomically replace the vault.
10. Sync the containing directory.
11. Release the lock.

Syncing the file alone does not ensure that its directory entry has reached persistent storage; Linux documents the need for a separate directory sync in [`fsync`](https://man7.org/linux/man-pages/man2/fsync.2.html).

Initialization uses publication that refuses replacement of an existing destination.

If failure occurs before replacement, preserve the original. If replacement succeeds but directory sync fails, report that the change may be present but durability could not be confirmed. Do not report ordinary success or blindly retry.

## 10. Clipboard Behavior

Keep clipboard handling behind a Linux-specific adapter:

- Wayland backend using `wl-copy`.
- X11 backend using `xclip`.
- Clear error on a headless session or missing dependency.

Pass the value through the child process's stdin. Never construct a shell command containing a secret.

Proposed behavior:

1. Unlock and find the entry.
2. Retain only the selected value for clipboard use.
3. Release the vault lock and discard other decrypted data.
4. Start a foreground clipboard owner.
5. Keep the command running for up to 30 seconds.
6. Stop its clipboard owner on expiry or interruption.

Cleanup must not overwrite newer clipboard content. Terminating the application's own clipboard source is preferable to issuing an unconditional delayed "clear clipboard."

Report this as best-effort clipboard expiry, not guaranteed erasure. Clipboard managers may retain a copy. Where supported, pass Wayland's sensitive-content hint; the [wl-clipboard documentation](https://github.com/bugaevc/wl-clipboard/blob/master/data/wl-clipboard.1) describes it as a hint that some managers honor.

## 11. Password Changes, Backups, and Restoration

### Password Change

`pbox passwd` should:

1. Unlock with the current password.
2. Request and confirm the new password.
3. Generate a new salt and nonce.
4. Derive a new key.
5. Re-encrypt the entire vault.
6. Save using the normal transaction.

Existing backups remain encrypted with their original passwords. Changing the current password does not revoke access to older copies.

### Backup

`pbox backup <destination>` should:

- Authenticate the source vault.
- Copy its encrypted snapshot.
- Refuse an existing destination.
- Apply private permissions.
- Sync the backup before reporting success.

Use explicit backups initially. Automatic retained snapshots complicate deletion expectations and password changes.

### Restore

`pbox restore <backup>` should:

1. Ask for the backup's password.
2. Authenticate and fully validate the backup.
3. Authenticate the current vault if one exists.
4. Confirm replacement.
5. Re-encrypt restored data with the current vault password and a fresh nonce.
6. Replace the current vault using the normal save transaction.

Restoration into an existing vault increments that destination's revision rather than trusting the backup revision as a rollback defense. Preserve the restored entries' timestamps.

If creating a new vault from backup, obtain creation consent and request a new master password.

## 12. Go Architecture

Use a small set of packages with explicit responsibilities:

```text
cmd/pbox/
  main.go

internal/
  cli/          Command parsing and presentation
  vault/        Entry rules and operation orchestration
  cryptobox/    Key derivation and authenticated encryption
  storage/      Envelope parsing, permissions, locking, durable writes
  terminal/     Hidden input and controlling-terminal handling
  clipboard/    Linux clipboard adapters

docs/
```

The dependency direction is:

```text
CLI -> vault operations -> cryptography and storage
CLI -> terminal and clipboard adapters
```

Cryptography should know nothing about CLI commands. Storage should persist encrypted bytes without understanding entry values.

Use interfaces where they provide a concrete boundary or allow failure injection, particularly clipboard operations and durable file writes. Avoid adding generic repository frameworks.

Proposed dependencies:

| Need | Choice |
| --- | --- |
| Commands and help | Cobra |
| Cryptography | `golang.org/x/crypto` |
| Hidden input | `golang.org/x/term` |
| Linux locking and file operations | `golang.org/x/sys/unix` |
| Encoding, randomness, process management | Go standard library |

Pin dependency versions when implementation begins.

## 13. Error Handling

Use typed internal errors and stable CLI exit codes:

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | I/O or unexpected operational failure |
| `2` | Invalid command or input |
| `3` | Unlock authentication failed |
| `4` | Entry not found |
| `5` | Entry already exists |
| `6` | Vault busy |
| `7` | Unsupported or malformed vault |
| `8` | Clipboard unavailable |
| `130` | Interrupted |

A declined destructive action leaves the vault unchanged and reports cancellation.

Never include secret values in errors or diagnostic output.

## 14. Step-by-Step Build Sequence

This is the proposed sequence; each step has a concrete completion condition.

For implementation, follow [PHASES.md](PHASES.md), which expands these broad steps into 14 smaller phases with learning notes and an explicit user review before advancing.

| Step | Work | Completion condition |
| --- | --- | --- |
| 1. Finalize the specification | Agree on commands, limits, file format, security boundaries, and platform scope | One consistent specification guides implementation |
| 2. Establish the Go CLI | Create the module, command tree, help, version output, and error mapping | Help and invalid-command behavior work |
| 3. Implement vault rules | Add the model, validation, CRUD operations, sorting, and timestamps | Business rules pass unit tests independently of disk access |
| 4. Implement cryptography | Add Argon2id, XChaCha20-Poly1305, random generation, and authenticated headers | Round trips, incorrect passwords, and tampering are tested |
| 5. Implement safe storage | Add envelope parsing, size limits, permissions, locking, and durable replacement | Concurrency and injected write failures preserve valid state |
| 6. Implement initialization | Add consent, hidden password prompts, validation, and no-overwrite creation | Refusal creates nothing; consent creates an unlockable empty vault |
| 7. Connect CRUD commands | Wire add, list, show, edit, delete, stdin input, and raw output | A full CLI lifecycle works without unintended secret output |
| 8. Add password changes | Re-encrypt with a new salt and key | The new password opens the current vault; the old one fails |
| 9. Add backup and restore | Implement authenticated snapshots and confirmed restoration | A backup can restore the expected entries safely |
| 10. Add clipboard adapters | Implement Wayland and X11 ownership, expiry, and cleanup | Copy works and expiry leaves newer clipboard content untouched |
| 11. Harden and verify | Fuzz parsers, test interruptions, inspect files, benchmark unlock, scan dependencies | Security and failure-path acceptance checks pass |
| 12. Package the Linux release | Build binaries, checksums, installation instructions, and recovery guidance | A clean Linux environment can install and use the documented workflow |

## 15. Required Verification

The most important tests are:

- Incorrect passwords and modified headers, ciphertext, or tags fail safely.
- Malformed files cannot request unbounded KDF resources.
- Two writers cannot lose each other's changes.
- Disk-full and interrupted-write failures preserve a valid vault.
- Initialization never replaces an existing vault.
- Test secrets and entry names do not appear in application-created disk files.
- Password changes and backup restoration behave as documented.
- Clipboard expiry does not erase a newer selection.
- Noninteractive execution never silently falls back to exposing a password.

Use unit tests, CLI integration tests, subprocess concurrency tests, injected storage failures, and parser fuzzing. Benchmark unlock time and memory on representative Linux machines before release.

## 16. Later Releases

After the Linux implementation meets the acceptance conditions, evaluate:

1. macOS support for private storage, locking, durable writes, terminal input, and clipboard operations.
2. Windows support using platform-appropriate access controls, file replacement, locking, and clipboard APIs.
3. Key renaming and additional discovery features based on user feedback.
4. An explicitly designed unlock session or operating-system keychain integration if repeated password prompts become a demonstrated problem.

Keep the vault format portable. Treat platform support as work that requires platform-specific verification, rather than assuming that a successful Go cross-compilation proves correct behavior.
