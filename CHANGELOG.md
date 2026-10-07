# Changelog

All notable changes to **Dev-Sync** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-10-07

### Added
- **Core Engine**: Application-agnostic, unidirectional `SOURCE → REPLICAS` synchronization engine.
- **Atomic Operations**: Safe chunked file streaming with atomic replacement semantics (`.devsync-tmp.*` -> `fsync` -> atomic rename).
- **Filesystem Safety Guard**: Strict path boundary resolution preventing traversal (`../`), destination overlap, and system root wiping.
- **Drift Detection & Auto-Repair**: Deep and size-based detection of replica drift with non-destructive source repair.
- **Real-Time Watch Mode**: Recursive filesystem watcher utilizing `fsnotify` with configurable debouncing (coalescing rapid editor temp file operations).
- **CLI Commands**: Full suite including `init`, `add`, `remove`, `list`, `sync`, `watch`, `status`, `verify`, `doctor`, `config`, and `version`.
- **Integrity Verification**: Cryptographic SHA-256 deep verification command (`dev-sync verify`).
- **Ignore Engine**: Project-level `.devsyncignore` support supporting wildcards, directory rules, recursive paths, and negation.
- **Logging**: Rotating file logger (`.devsync/logs/dev-sync.log`) with automatic archiving and ANSI terminal output.
- **Discord Integration**: Webhook notification client supporting rich embeds, event filters, rate limiting, and secret masking.
- **Cross-Platform Compatibility**: Tested across Windows (x64, ARM64) and Linux Debian/RHEL distributions (x64, ARM64).
