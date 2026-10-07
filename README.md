# Dev-Sync

> **One Source. Multiple Replicas. Zero Hassle.**  
> Continuously synchronize a single source directory with multiple target directories in real time.

```text
                         SOURCE (Authoritative)
                                   │
                     ┌─────────────┼─────────────┐
                     │             │             │
                     ▼             ▼             ▼
                  Replica 1     Replica 2     Replica 3
```

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20(macOS%20compatible)-lightgrey.svg)](#1-install)
[![Dependencies](https://img.shields.io/badge/Dependencies-Zero%20(Single%20Binary)-brightgreen.svg)](#1-install)

---

## ⚡ Quick Start (60-Second Plug & Play)

### 1. Install

**Windows (PowerShell)**:
```powershell
# Download standalone dev-sync.exe
Invoke-WebRequest -Uri "https://github.com/devsync/dev-sync/releases/latest/download/dev-sync.exe" -OutFile "$HOME\dev-sync.exe"
# (Optional) Add to PATH so you can use it anywhere:
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";$HOME", "User")
```

**Linux (Ubuntu, Debian, RHEL, Rocky, CentOS - x64 & ARM64)**:
```bash
# Option A: One-line install (auto-detects x64 / ARM64):
curl -fsSL https://raw.githubusercontent.com/devsync/dev-sync/main/scripts/install.sh | bash

# Option B: Self-extracting .run package from GitHub Releases:
chmod +x dev-sync-linux-arm64.run && sudo ./dev-sync-linux-arm64.run
```

**macOS**:
```bash
curl -sSL https://github.com/devsync/dev-sync/releases/latest/download/dev-sync-darwin-arm64 -o dev-sync
sudo install -m 755 dev-sync /usr/local/bin/dev-sync
```

---

### 2. Configure Your Project

Navigate to your source code directory and initialize:

```bash
cd /path/to/my-project
dev-sync init
```

Follow the 10-second prompt:
- **Source**: Press <kbd>Enter</kbd> (defaults to current folder `.`)
- **Add replica**: Type `y`, then paste your replica destination path (e.g. `C:\Servers\test-env` or `/opt/staging`)
- Done! Sensible ignore rules (`.git/`, `node_modules/`, logs, and temp files) are configured automatically.

---

### 3. Run & Forget

```bash
dev-sync watch
```

**That's it!** Whenever you edit or delete files in your source directory, Dev-Sync immediately mirrors them to all your replicas.

*(Need a one-time sync instead? Just run `dev-sync sync`)*

---

## 🛠️ Command Cheat Sheet

| Command | Action |
|---|---|
| `dev-sync watch` | Continuously mirrors source changes to replicas in real time |
| `dev-sync sync` | Performs a one-time synchronization pass |
| `dev-sync sync --dry-run` | Previews planned changes without touching any files on disk |
| `dev-sync status` | Checks if replicas are in sync, missing files, or drifted |
| `dev-sync add` | Adds an additional replica folder to an existing project |
| `dev-sync remove` | Removes a replica from sync config (never touches your files) |
| `dev-sync list` | Displays all configured source and replica destinations |
| `dev-sync verify` | Deep cryptographic SHA-256 verification across all files |
| `dev-sync doctor` | Runs health diagnostics on permissions, paths, and watchers |
| `dev-sync config` | Prints current configuration settings |

---

## 📖 Detailed Overview & Architecture

### Why Dev-Sync?
Dev-Sync is built for developer workflows where code must physically exist in multiple places:
- **Game / Server Development** (FiveM, Minecraft servers, game engines requiring scripts placed directly in runtime directories)
- **Multi-Environment Testing** (Simultaneously testing local, staging, and container mounts)
- **Local Deployment Clusters** (Multiple microservice instances without slow build/copy loops)

---

### Core Guarantees

#### 1. Unidirectional Authority (`SOURCE ➔ REPLICAS`)
The source directory is always authoritative. Changes in replicas are classified as **drift** and are never copied back into your source code.

#### 2. Atomic File Replacement
Files are never partially written or locked. Dev-Sync streams to temporary files (`.devsync-tmp.*`), flushes storage buffers to disk (`fsync`), verifies integrity, and executes an atomic replace.

#### 3. Path Traversal & Root Safety
Target paths are validated to stay strictly inside replica boundaries (`SafeResolvePath`). Dangerous directory traversals (`../`), overlapping source/replica loops, and system roots (`/`, `C:\`, `C:\Windows`) are actively blocked.

#### 4. Smart Event Coalescing
Modern code editors (VS Code, JetBrains, Vim) generate multiple rapid events per save (temp write ➔ rename ➔ modify). Dev-Sync debounces events (default `150ms`) into a single logical update.

---

### 🛡️ Drift Detection & Auto-Repair

If a replica is modified externally or extra files are added:

```bash
dev-sync status
```
```text
my-replica
⚠ OUT OF SYNC
  Modified: src/main.js
  Extra:    debug.log
```

To restore the replica from authoritative source:
```bash
# Manual repair:
dev-sync sync --replica my-replica

# Or auto-repair on the fly during watch mode:
dev-sync watch --auto-repair
```

---

### 🩺 Health Checks & Verification

- **`dev-sync doctor`**: Pre-flight diagnostic tool checking disk access, write permissions, watcher availability, and configuration integrity.
- **`dev-sync verify`**: Performs deep byte-by-byte SHA-256 hashing across source and replicas to ensure exact parity.

---

### ⚙️ Full Configuration Reference (`.devsync/config.json`)

```json
{
  "version": 1,
  "source": "/path/to/source",
  "replicas": [
    {
      "id": "staging-env",
      "path": "/path/to/replica",
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
  "filesystem": {
    "symlinkMode": "skip",
    "bufferSizeKB": 64
  },
  "discord": {
    "enabled": false,
    "webhookUrl": ""
  }
}
```

---

### 🔔 Optional Discord Webhooks

Send sync alerts and drift notifications to a Discord channel:
1. In `.devsync/config.json`, set `"discord": { "enabled": true }`.
2. Provide your webhook URL via config or environment variable:
   ```bash
   export DEVSYNC_DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/..."
   ```
*(Webhook URLs are always masked in logs as `https://discord.com/api/webhooks/.../********` to protect credentials).*

---

### 🔨 Building from Source

Requires Go 1.22+:

```bash
git clone https://github.com/devsync/dev-sync.git
cd dev-sync
go build -o dev-sync ./cmd/dev-sync
```

---

## License

Released under the [MIT License](LICENSE).
