<p align="center">
  <img src="docs/banner.png" alt="SiloBang" width="700">
</p>

<p align="center">
  <strong>Self-hosted, content-addressed asset storage with cryptographic integrity verification.</strong>
</p>

<p align="center">
  <a href="https://github.com/Fantasim/silobang/releases">Download</a> &middot;
  <a href="#configuration">Configuration</a> &middot;
  <a href="#installation">Installation</a>
</p>

---

## What is SiloBang?

SiloBang is a self-hosted storage system that guarantees file integrity through content-addressed hashing. Every file is identified by its BLAKE3 cryptographic hash and stored in DAT archive containers that can be independently verified at any time.

It ships as a **single binary** with an embedded web interface — no external dependencies, no database server, no runtime to install.

## Features

- **Content-addressed storage** — Files are deduplicated and identified by their BLAKE3 hash
- **Integrity verification** — Verify any file, topic, or your entire archive against stored hashes at any time
- **Topic-based organization** — Group assets into topics, each with its own DAT files and metadata database
- **Granular access control** — 10 permission actions with per-user constraints and daily quotas
- **Audit logging** — Every operation is logged with who, what, and when
- **Query engine** — Built-in query presets for time-series analysis, size distribution, recent imports, and more
- **Bulk operations** — Batch metadata edits, bulk downloads as ZIP with progress streaming
- **Single binary** — Frontend is embedded in the Go binary. Download, run, done.
- **Cross-platform** — Linux, macOS, and Windows (amd64 & arm64)

## Quick Start

```bash
# Download the latest release for your platform
# https://github.com/Fantasim/silobang/releases

# Extract and run
tar -xzf silobang-*.tar.gz
./silobang

# Open your browser
# http://localhost:2369
```

On first launch, SiloBang will guide you through creating your admin account and setting a working directory.

## Installation

### Download a release (recommended)

Grab the latest binary from the [Releases page](https://github.com/Fantasim/silobang/releases). Binaries are available for:

| Platform       | Architecture |
|----------------|-------------|
| Linux          | x86-64      |
| Linux          | ARM64       |
| macOS          | x86-64      |
| macOS          | ARM64       |
| Windows        | x86-64      |

Each release includes SHA256 checksums for verification.

### Build from source

**Requirements:** Go 1.25+, Node.js 20+, Make

```bash
git clone https://github.com/Fantasim/silobang.git
cd silobang
make build
./silobang
```

This builds the frontend, embeds it into the Go binary, and produces a standalone `silobang` executable.

## Configuration

SiloBang stores its configuration at:

```
~/.config/silobang/config.yaml
```

A default config is created automatically on first run. All fields are optional — sensible defaults are applied for anything you don't set.

```yaml
# Where your topics, DAT files, and databases are stored.
# Set this through the web UI on first run, or specify it here.
working_directory: "/path/to/your/data"

# HTTP server port (default: 2369)
port: 2369

# Maximum size of a single DAT container file (default: 1GB).
# When a DAT file reaches this size, a new one is created.
max_dat_size: 1073741824

# Maximum total disk usage across all topics.
# Set to 0 for unlimited (default), otherwise must be >= 1GB.
max_disk_usage: 0

# Authentication & session settings
auth:
  max_login_attempts: 5         # Failed attempts before lockout
  lockout_duration_mins: 15     # Lockout duration after max attempts
  session_duration_hours: 24    # Session lifetime
  session_max_duration_hours: 168  # Absolute max even with refresh (7 days)

# Bulk download settings
bulk_download:
  session_ttl_mins: 120         # Download session expiration
  max_assets: 900000000         # Max files per bulk download

# Audit log management
audit:
  max_log_size_bytes: 10737418240  # Max log size before purge (10GB)
  purge_percentage: 5              # Remove oldest N% when limit reached

# Per-asset metadata limits
metadata:
  max_value_bytes: 10485760     # Max size per metadata value (10MB)

# Batch operation limits
batch:
  max_operations: 100000        # Max metadata ops per request

# Monitoring settings
monitoring:
  log_file_max_read_bytes: 5242880  # Max log read size in UI (5MB)
```

### Key configuration notes

- **`working_directory`** is the most important setting — it's where all your data lives. You can set it via the web UI on first launch or directly in the config file.
- **`max_dat_size`** controls when DAT container files roll over. Larger values mean fewer files; smaller values are easier to back up individually.
- **`max_disk_usage`** provides a safety net to prevent filling your disk. When set, SiloBang will reject uploads that would exceed this limit.
- All other settings have reasonable defaults and rarely need changing.

## First Run

1. Start SiloBang: `./silobang`
2. Open `http://localhost:2369` in your browser
3. You'll be prompted to **create an admin account** (username + password, min 12 characters)
4. Set your **working directory** — the folder where SiloBang will store all topics and data
5. Start creating topics and uploading assets

## How It Works

```
working_directory/
  my-topic/
    000001.dat          # DAT container (assets + BLAKE3 hashes)
    000002.dat          # Next container when 000001 is full
    .internal/
      my-topic.db       # Topic metadata (SQLite)

~/.config/silobang/
  config.yaml           # Configuration
  .internal/
    orchestrator.db     # Users, permissions, audit logs (SQLite)
```

Each **topic** is a folder containing DAT files. Each DAT file stores assets alongside their BLAKE3 hash headers. When you run **verification**, SiloBang re-hashes every stored asset and compares it against the recorded hash — any mismatch is flagged immediately.

## License

See [LICENSE](LICENSE) for details.
