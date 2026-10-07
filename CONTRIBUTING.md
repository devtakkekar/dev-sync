# Contributing to Dev-Sync

Thank you for your interest in contributing to **Dev-Sync**! We welcome code contributions, bug reports, feature suggestions, and documentation improvements.

## Code of Conduct

Please treat everyone with respect and follow standard open-source civility guidelines.

## Development Setup

Dev-Sync is written in Go and requires Go 1.22+.

### Building from Source

```bash
git clone https://github.com/devsync/dev-sync.git
cd dev-sync
go build -o dev-sync ./cmd/dev-sync
```

### Running Tests

Execute all unit and integration tests:

```bash
go test -v ./...
```

Run tests with the race detector:

```bash
go test -race ./...
```

## Pull Request Guidelines

1. **Safety First**: Synchronization correctness and filesystem safety take priority over premature optimizations. Never bypass path boundary validations or atomic writes.
2. **Cross-Platform**: All code must function cleanly across Windows (NTFS) and Linux (ext4, tmpfs). Avoid assuming path separators or case sensitivity.
3. **Tests Required**: Every new feature or bug fix must include corresponding automated tests.
4. **Clean Commits**: Write clear, descriptive commit messages.
