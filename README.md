# fastsync

⚡ **fastsync** is a fast, cross-platform file synchronization tool inspired by rsync,
designed for modern systems and simpler workflows.

It works seamlessly on **Linux, macOS, and Windows**, without relying on rsync or platform-specific hacks.

[中文文档](README.zh.md)

---

## Why fastsync?

rsync is powerful, but it comes with trade-offs:

- Painful setup on Windows
- Complex flags and legacy behavior
- Server-side configuration is not always straightforward

**fastsync** focuses on being:

- 🚀 **Cross-platform by default** — no WSL, no cygwin
- 🧩 **Modular** — daemon + instances, clean separation
- ⚙️ **Easy to configure** — simple TOML config
- 🔐 **Secure** — authentication and IP access control

---

## Features

- **Cross-Platform**: Linux / macOS / Windows
- **Two Modes**
  - **Daemon Mode**: Run as a long-lived sync server
  - **CLI Mode**: One-off local or remote synchronization
- **Efficient Transfer**
  - Incremental sync
  - Optional compression
- **Security**
  - Password authentication
  - IP allow / deny lists
- **Flexible**
  - File exclusion rules
  - Attribute preservation
  - Per-instance logging

---

## Installation

Build from source:

```bash
git clone https://github.com/taurusxin/fastsync.git
cd fastsync
go build -o fastsync ./cmd/fastsync
```

### Docker Deployment

You can also run Fastsync using Docker.

1. **Prepare Configuration**:
    Create a `config.toml` file. You can base it on `config.toml.example`. Ensure paths in config point to `/data` or other mounted volumes.

    ```toml
    # Example snippet for docker config.toml
    [instances]
    path = "/data"  # Map to volume inside container
    ```

2. **Run Container**:

    ```bash
    docker run -d \
      --name fastsync \
      -p 7963:7963 \
      -v $(pwd)/config.toml:/config/config.toml \
      -v $(pwd)/data:/data \
      -v $(pwd)/logs:/logs \
      taurusxin/fastsync:latest
    ```

## Quickstart

### 1. Daemon Mode (Server)

If you don't plan to deploy with Docker, you can start the server with a configuration file:

```bash
./fastsync -c config.toml
```

See [Configuration](#configuration) for details on `config.toml`.

### 2. Normal Mode (Client)

Synchronize files between source and target.

**Syntax:**

```bash
fastsync source target [options]
```

- **Source/Target**: Can be a local path or a remote address.
  - Local: `/path/to/dir`
  - Remote: `password@ip:port/instance_name` (Default instance is `default`)
  - *Note*: At least one path must be local.

**Options:**

- `-d`: **Delete**. Delete files in target that are missing in source.
- `-o`: **Overwrite**. Overwrite existing files in target (default is to skip if size/time matches).
- `-s`: **Checksum**. Use content hashing to detect changes (slower but more accurate).
- `-z`: **Compress**. Enable zlib compression during transfer.
- `-a`: **Archive**. Preserve file attributes (permissions, modification time).
- `-v`: **Verbose**. Print detailed logs during synchronization.

**Examples:**

```bash
# Local to Local
./fastsync ./source ./target -v -a

# Local to Remote (Push)
./fastsync ./source secret@192.168.1.100:7963/backup -z

# Remote to Local (Pull)
./fastsync secret@192.168.1.100:7963/backup ./restore -d -a
```

**Note: At least one path must be local.**

## Configuration

A sample configuration file (`fastsync.toml.example`) is provided.

**Global Settings:**

- `address`: Bind address (default "127.0.0.1").
- `port`: Listening port (default 7963).
- `log_level`: Global log level (info, warn, error).
- `log_file`: Path to global log file.

**Instance Settings:**

- `name`: Unique name for the sync module.
- `path`: Local file system path to serve.
- `password`: Authentication password.
- `exclude`: Comma-separated list of glob patterns to ignore.
- `host_allow` / `host_deny`: CIDR IP lists for access control.
- `log_level`: Instance log level.
- `log_file`: Path to instance log file.

## Roadmap

- [ ] Configuration file hot reload
- [ ] Incremental sync
- [ ] File encryption
- [ ] Resume interrupted transfers
- [ ] Multi-point chunked transfers for large files
