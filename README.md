# Dev-Sync

> **One Source. Multiple Replicas. Zero Hassle.**  
> A lightweight, cross-platform developer tool that continuously synchronizes a single source folder to multiple target folders in real time.

```text
                         SOURCE (Authoritative)
                                   │
                     ┌─────────────┼─────────────┐
                     │             │             │
                     ▼             ▼             ▼
                  Replica 1     Replica 2     Replica 3
```

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg)](#installation)
[![Zero Dependencies](https://img.shields.io/badge/Dependencies-Zero%20(Single%20Binary)-brightgreen.svg)](#overview)

---

## Why Dev-Sync?

If you've ever had to copy code between multiple folders—such as:
- Multiple local development or sandbox environments
- Game server directories (FiveM, Minecraft, game engines) where code must live physically inside runtime folders
- Testing simultaneous builds or local staging instances
- Microservice clusters or multi-tenant folders

...Dev-Sync runs quietly in the background and keeps your targets synchronized the second you save a file.

- **Unidirectional Safety (`Source ➔ Replicas`)**: Your source folder is strictly authoritative. Changes inside a replica are never copied back to your source code.
- **Atomic & Safe**: Files are never partially written or corrupted. Dev-Sync writes to temporary staging files, syncs to disk, and replaces files atomically.
- **Language & Framework Agnostic**: Works with everything—Node.js, Python, Go, Rust, PHP, C++, static assets, or game files.
- **Zero Configuration Bloat**: No accounts, no daemons, no databases, no runtimes. Download the single executable and run.

---

## Quick Start (Plug & Play)

### Step 1: Install

#### Windows
Download **`dev-sync.exe`** from [Releases](https://github.com/devsync/dev-sync/releases/latest) and place it in your project or add it to your PATH:

```powershell
# Optional: Add to PATH in PowerShell
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\path\to\dev-sync", "User")
```

#### Linux & macOS
```bash
curl -sSL https://github.com/devsync/dev-sync/releases/latest/download/dev-sync-linux-amd64.tar.gz | tar -xz
sudo install -m 755 dev-sync-linux-amd64 /usr/local/bin/dev-sync
```

---

### Step 2: Initialize Your Project (30 Seconds)

Navigate into your source project directory and run:

```bash
dev-sync init
```

An interactive setup will ask you:
1. **Source directory**: (Press Enter for current directory `.`)
2. **Add replica**: Enter `y` and provide the destination folder path (e.g. `C:\Servers\test-env` or `/opt/staging`).
3. Press Enter to confirm!

> 💡 **Sensible Defaults:** Dev-Sync automatically generates a `.devsyncignore` file. Common files like `.git/`, `node_modules/`, editor temp files, and logs are automatically excluded out of the box.

---

### Step 3: Run & Develop

You're done! Now start watch mode:

```bash
dev-sync watch
```

Whenever you save or delete a file in your source folder, Dev-Sync immediately mirrors the change to all your replicas.

To run a one-time synchronization instead:

```bash
dev-sync sync
```

---

## Daily Commands Cheat Sheet

| Command | What it does |
|---|---|
| `dev-sync watch` | Continuously mirrors changes to replicas in real time |
| `dev-sync sync` | Runs a one-time synchronization pass |
| `dev-sync sync --dry-run` | Previews planned changes without touching any files on disk |
| `dev-sync status` | Checks if replicas are up-to-date, missing files, or drifted |
| `dev-sync add` | Adds a new replica destination folder |
| `dev-sync remove` | Removes a replica from configuration (never deletes your files) |
| `dev-sync list` | Shows your configured source and replicas |
| `dev-sync verify` | Deep cryptographic SHA-256 integrity check across all files |
| `dev-sync doctor` | Checks disk permissions, watcher capabilities, and health |

---

## Common Features

### 🛡️ Out-of-Sync Detection & Auto-Repair
If a file inside a replica is modified or an extra file is placed there accidentally:
```bash
dev-sync status
```
Outputs:
```text
my-replica
⚠ OUT OF SYNC
  Modified: src/main.js
  Extra:    debug.log
```

To automatically restore the replica back to the source:
```bash
# Manual repair:
dev-sync sync --replica my-replica

# Or let watch mode automatically heal drift on the fly:
dev-sync watch --auto-repair
```

### ⚡ Smart Save Coalescing
Code editors (VS Code, JetBrains, Sublime, Vim) often trigger multiple filesystem events on a single save (temp file write ➔ rename ➔ save). Dev-Sync debounces rapid events (default `150ms`) so only one clean, aggregated sync occurs.

### 🔔 Optional Discord Notifications
Dev-Sync can optionally post sync summaries and alerts to a Discord webhook:
1. In `.devsync/config.json`, set `"discord": { "enabled": true }`.
2. Provide your webhook via config or environment variable:
   ```bash
   export DEVSYNC_DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/..."
   ```
*(Webhook URLs are always masked in logs to protect your secrets).*

---

## Configuration (`.devsync/config.json`)

Stored in your project's root folder under `.devsync/config.json`:

```json
{
  "version": 1,
  "source": "C:\\Projects\\my-app",
  "replicas": [
    {
      "id": "staging-server",
      "path": "C:\\Servers\\staging\\my-app",
      "enabled": true
    }
  ],
  "sync": {
    "deleteExtraneous": true,
    "verifyAfterSync": true,
    "atomicWrites": true
  },
  "drift": {
    "autoRepair": false
  },
  "watch": {
    "debounceMs": 150
  }
}
```

---

## Building from Source

Requires Go 1.22+:

```bash
git clone https://github.com/devsync/dev-sync.git
cd dev-sync
go build -o dev-sync ./cmd/dev-sync
```

---

## License

Released under the [MIT License](LICENSE).
