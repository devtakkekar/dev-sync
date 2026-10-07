# Dev-Sync

> **Dev-Sync is a lightweight, cross-platform developer utility that keeps one source directory synchronized with multiple physical replica directories.**

```text
                         SOURCE
                            │
              ┌─────────────┼─────────────┐
              │             │             │
              ▼             ▼             ▼
           REPLICA 1     REPLICA 2     REPLICA 3
```

[![CI/CD](https://github.com/devsync/dev-sync/actions/workflows/ci.yml/badge.svg)](https://github.com/devsync/dev-sync/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8.svg?logo=go)](https://go.dev)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg)](#supported-operating-systems)

---

## Table of Contents

- [Overview](#overview)
- [Core Principles](#core-principles)
- [Key Features](#key-features)
- [Supported Platforms](#supported-platforms)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [CLI Command Reference](#cli-command-reference)
- [Configuration (`config.json`)](#configuration)
- [Ignore Rules (`.devsyncignore`)](#ignore-rules-devsyncignore)
- [Watch Mode & Debouncing](#watch-mode--debouncing)
- [Drift Detection & Auto-Repair](#drift-detection--auto-repair)
- [Discord Notifications](#discord-notifications)
- [Security Guarantees](#security-guarantees)
- [Architecture](#architecture)
- [Building from Source](#building-from-source)
- [Exit Codes](#exit-codes)
- [License](#license)

---

## Overview

Modern software development often requires one authoritative source directory to physically mirror across multiple target locations:
- **Multiple development environments** (local, test, sandbox)
- **Local deployment instances** (server clusters, microservice staging paths)
- **Game/server development** (FiveM, Minecraft servers, game engines requiring files located inside engine runtime directories)
- **Cross-environment verification** without git branches or complex symlink loops

Dev-Sync solves this problem with a clean, standalone, zero-dependency executable designed for maximum correctness and safety.

---

## Core Principles

### 1. Directional Immutability (`SOURCE → REPLICAS`)
The source directory is always **authoritative**. Changes in source flow to replicas. Changes made directly inside a replica are classified as **drift** and are never copied back into the source.

### 2. Framework & Language Agnostic
Dev-Sync has zero assumptions about Node.js, Python, Go, Java, game servers, databases, or IDEs. It is purely a filesystem synchronization engine.

### 3. Transaction-Like Atomic Writes
Files are never partially written directly to target locations. Dev-Sync streams to temporary files (`.devsync-tmp.*`) within destination directories, flushes storage buffers (`fsync`), verifies integrity, and executes atomic filesystem replaces.

### 4. Boundary Protection
Dev-Sync verifies that operations remain strictly inside replica boundaries, preventing path traversal (`../`), root wipes, or accidental overwrite of the source.

---

## Key Features

- 🚀 **High Performance**: Streaming I/O with 64KB/1MB buffers; handles projects with tens of thousands of files with low CPU and memory footprints.
- ⚡ **Smart Change Detection**: Skips unchanged files using size and modification timestamp heuristics, with fallback to streaming SHA-256 hashes.
- 🔄 **Real-Time Watch Mode**: Recursive filesystem watching powered by `fsnotify`, with intelligent debouncing to coalesce rapid editor save cycles.
- 🛡️ **Drift Detection & Repair**: Identifies replica-side modifications or extraneous files, restoring replicas to match authoritative source on demand or automatically.
- 🔍 **Deep Cryptographic Verification**: `dev-sync verify` computes full SHA-256 hashes across source and replicas to guarantee binary identity.
- 🩺 **Built-in Diagnostics**: `dev-sync doctor` performs comprehensive pre-flight health checks across permissions, disks, configs, and watchers.
- 💬 **Discord Webhooks**: Optional, rate-limited notifications with rich embed messages for sync completions, failures, and drift detections.
- 🔒 **Zero Telemetry & Offline First**: Works 100% offline; secrets and webhook URLs are masked across logs and terminals.

---

## Supported Operating Systems

- **Windows**: Windows 10, Windows 11, Windows Server (x64, ARM64)
- **Linux (Debian-based)**: Debian 11/12, Ubuntu 20.04/22.04/24.04 (x64, ARM64)
- **Linux (RHEL-based)**: RHEL 8/9, Rocky Linux 8/9, AlmaLinux 8/9, CentOS Stream 9 (x64, ARM64)
- **macOS**: macOS 12+ (Apple Silicon & Intel)

---

## Installation

### Pre-built Binaries

Download the standalone binary from the [Releases](https://github.com/devsync/dev-sync/releases) page:

#### Windows
1. Download `dev-sync-windows-amd64.zip`.
2. Extract `dev-sync.exe` into a directory included in your system `%PATH%` (e.g. `C:\Tools\dev-sync.exe`).

#### Linux
```bash
# Download and install to /usr/local/bin
curl -sSL https://github.com/devsync/dev-sync/releases/latest/download/dev-sync-linux-amd64.tar.gz | tar -xz
sudo install -m 755 dev-sync-linux-amd64 /usr/local/bin/dev-sync
```

---

## Quick Start

### 1. Initialize your Project

Navigate to your authoritative source project directory:

```bash
cd /path/to/my-project
dev-sync init
```

The interactive wizard guides you through registering replica destinations:

```text
╔══════════════════════════════════════════╗
║                 Dev-Sync                 ║
║     One Source. Multiple Replicas.       ║
╚══════════════════════════════════════════╝

Initialize a synchronization project

Source directory [default: .]:
> .

Add replica? [Y/n]:
> y
Replica name/ID:
> staging
Replica directory path:
> /opt/apps/staging/my-project

Add replica? [Y/n]:
> n

✓ Configuration created successfully in /path/to/my-project/.devsync
```

### 2. Synchronize

Run an initial dry-run or live sync:

```bash
# Preview actions without modifying disk
dev-sync sync --dry-run

# Execute synchronization
dev-sync sync
```

### 3. Continuous Watch

Keep replicas continuously synchronized as you write code:

```bash
dev-sync watch
```

---

## CLI Command Reference

| Command | Description |
|---|---|
| `dev-sync init` | Initialize a synchronization project configuration |
| `dev-sync add` | Register a new replica without re-initializing |
| `dev-sync remove` | Remove a replica from configuration (retains physical files) |
| `dev-sync list` | Display source and all configured replicas |
| `dev-sync sync` | Compare and synchronize source files to replicas |
| `dev-sync watch` | Continuously monitor source for changes and mirror in real time |
| `dev-sync status` | Check synchronization state (`IN SYNC`, `OUT OF SYNC`, `UNAVAILABLE`) |
| `dev-sync verify` | Deep cryptographic SHA-256 verification |
| `dev-sync doctor` | Diagnostic checks across filesystem permissions, watcher, and config |
| `dev-sync config` | Display active configuration JSON (with sanitized secrets) |
| `dev-sync version` | Show version, platform architecture, and build commit |
| `dev-sync help` | Display CLI help and options |

### Command Flags

#### `dev-sync sync`
- `--replica <id>`: Synchronize only the specified replica ID.
- `--dry-run`: Compute the full plan and statistics without writing to disk.
- `--deep`: Force full SHA-256 hash comparison on all files.
- `--verbose`: Print per-file actions as they are executed.
- `--json`: Output execution metrics in JSON format for automation/scripts.

#### `dev-sync watch`
- `--debounce <ms>`: Debounce window in milliseconds (default: `150ms`).
- `--replica <id>`: Mirror changes only to a specific replica.
- `--auto-repair`: Automatically repair detected replica drift from authoritative source.

---

## Configuration

Project configuration resides in `.devsync/config.json`:

```json
{
  "version": 1,
  "source": "/home/user/projects/my-project",
  "replicas": [
    {
      "id": "environment-a",
      "path": "/home/user/test/environment-a",
      "enabled": true
    },
    {
      "id": "environment-b",
      "path": "/home/user/test/environment-b",
      "enabled": true
    }
  ],
  "sync": {
    "deleteExtraneous": true,
    "verifyAfterSync": true,
    "atomicWrites": true,
    "preserveTimestamps": true,
    "preservePermissions": true
  },
  "drift": {
    "autoRepair": false,
    "detectOnWatch": true
  },
  "watch": {
    "debounceMs": 150,
    "source": true,
    "replicas": false
  },
  "verification": {
    "enabled": false,
    "intervalSeconds": 300
  },
  "filesystem": {
    "symlinkMode": "skip",
    "bufferSizeKB": 64
  },
  "discord": {
    "enabled": false,
    "webhookUrl": "",
    "events": {
      "syncStarted": false,
      "syncCompleted": true,
      "syncFailed": true,
      "driftDetected": true,
      "driftRepaired": true,
      "replicaUnavailable": true
    }
  }
}
```

---

## Ignore Rules (`.devsyncignore`)

Place a `.devsyncignore` file in your source root to exclude files or directories. Dev-Sync follows gitignore-compatible patterns:

```gitignore
# Exclude build output and dependencies
node_modules/
dist/
vendor/

# Temporary and log files
*.log
*.tmp
*.bak

# OS metadata
.DS_Store
Thumbs.db

# Negation: keep this specific log file
!important.log
```

> **Note**: `.devsync/`, `.git/`, and temporary working files (`*.devsync-tmp.*`) are **always** protected and ignored automatically.

---

## Watch Mode & Debouncing

Code editors and IDEs (VS Code, JetBrains, Vim) frequently generate rapid bursts of events upon saving:
```text
Write temporary file -> Rename -> Modify -> Delete old backup
```

Dev-Sync's watcher coalesces these rapid bursts into a single logical change set using configurable debouncing (`debounceMs: 150`), preventing redundant writes or churn.

---

## Drift Detection & Auto-Repair

If a replica is modified out-of-band:
```bash
dev-sync status
```
Outputs:
```text
development
✓ IN SYNC

testing
⚠ OUT OF SYNC
  Modified:
    src/main.js
  Extra:
    debug.log
```

To repair drift non-destructively from authoritative source:
```bash
# Manual repair of specific replica
dev-sync sync --replica testing

# Or enable auto-repair in watch mode
dev-sync watch --auto-repair
```

---

## Discord Notifications

Enable webhook notifications by setting `discord.enabled: true` in `.devsync/config.json` or providing the environment variable:

```bash
export DEVSYNC_DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/..."
```

Dev-Sync features:
- **Rich Embeds** with color coding (Green for success, Red for failure, Orange for drift).
- **Notification Aggregation**: Prevents webhook spam by batching watch mode updates.
- **Rate-Limit Resilience**: Respects Discord `Retry-After` headers with exponential backoff.
- **Secret Sanitization**: Webhook URLs are masked as `https://discord.com/api/webhooks/.../********` in all logs.

---

## Security Guarantees

1. **Strict Root Boundary**: Prevents directory traversal attacks (`../../`).
2. **System Directory Protection**: Forbids syncing into OS roots (`/`, `C:\`, `C:\Windows`, etc.).
3. **No Upstream Mutation**: Replicas never overwrite source data.
4. **Symlink Loop Prevention**: Symlinks and Windows junctions are skipped by default (`symlinkMode: "skip"`).

---

## Building from Source

Requirements: Go 1.22+

```bash
# Clone
git clone https://github.com/devsync/dev-sync.git
cd dev-sync

# Build native binary
go build -o dev-sync ./cmd/dev-sync

# Cross-compile for Linux x64
GOOS=linux GOARCH=amd64 go build -o dev-sync-linux-amd64 ./cmd/dev-sync

# Cross-compile for Linux ARM64
GOOS=linux GOARCH=arm64 go build -o dev-sync-linux-arm64 ./cmd/dev-sync
```

---

## Exit Codes

Dev-Sync uses deterministic, script-friendly exit codes:

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | General error |
| `2` | Configuration error |
| `3` | Validation / drift error |
| `4` | Synchronization failure |
| `5` | Verification failure |
| `6` | Replica unavailable |

---

## License

MIT License. See [LICENSE](LICENSE) for details.
