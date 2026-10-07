# Security Policy

## Reporting Security Vulnerabilities

If you discover a security vulnerability within **Dev-Sync**, please report it responsibly by contacting the maintainers directly or opening a confidential security advisory on GitHub.

Please **do not** report security vulnerabilities via public GitHub issues.

### Response Timeline
- **Acknowledgement**: Within 48 hours.
- **Triage & Assessment**: Within 5 business days.
- **Patch & Advisory**: Dependent on severity, typically within 14 days.

---

## Security Architecture & Design Principles

Dev-Sync operates with strict filesystem safety guarantees designed to protect your source code and host operating system.

### 1. Directional Immutability (`SOURCE → REPLICAS`)
- The source directory is strictly authoritative.
- Dev-Sync **never** copies modified files or extra files from a replica back into the source.
- Replica modifications are treated as drift and can either be reported or repaired by overwriting from the source.

### 2. Path Traversal & Boundary Safety
- All replica targets are normalized into absolute, canonical filesystem paths.
- Every read, write, and deletion verifies that target paths are strictly contained within their designated root directory using canonicalized subpath assertions (`SafeResolvePath`).
- Dangerous directory traversal sequences (`../`, nested parent escaping, raw root wipes) are actively blocked with `ErrPathEscape`.

### 3. Deletion Protection
- When `deleteExtraneous` is enabled, Dev-Sync only deletes files within configured replica directories that are absent in the source.
- Root filesystems (`/`, `C:\`, `C:\Windows`, `/etc`, `/bin`, etc.) are forbidden as replica roots.
- Dev-Sync explicitly forbids setting the replica directory to the source directory or any parent of the source directory.

### 4. Atomic File Replacement
- Files written to replicas are first streamed to temporary files (`.devsync-tmp.XXXXX`) created in the destination directory.
- Buffers are flushed and synchronized to persistent storage (`fsync`) before performing an atomic rename (`os.Rename` / `MoveFileExW`).
- This prevents corrupted or partially written files if an operation is interrupted or if a reader concurrently accesses the replica.

### 5. Symlink & Junction Safety
- By default, Dev-Sync skips symlinks and Windows junctions (`symlinkMode: "skip"`) to prevent symlink race conditions, infinite loops, or writes escaping into unintended directories.

### 6. Secret Masking
- Discord webhook URLs and credentials configured in `.devsync/config.json` or provided via environment variables (`DEVSYNC_DISCORD_WEBHOOK_URL`) are sanitized in logs, terminal outputs, and diagnostics.
- Webhook URLs are never exposed in plaintext diagnostics or crash reports.
