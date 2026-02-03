# Fastsync

[中文文档](README.zh.md)

**Fastsync** is a cross-platform, high-performance file synchronization tool designed to be a modern alternative to rsync. It supports both local and remote file synchronization with a modular design and easy configuration.

## Features

* **Cross-Platform**: Works on Linux, macOS, and Windows.
* **Dual Modes**:
  * **Daemon Mode**: Runs as a server, configured via TOML.
  * **Normal Mode**: Runs as a CLI tool for ad-hoc syncs (local-local or local-remote).
* **Efficient**: Supports incremental sync, multi-threaded transfer, and compression.
* **Secure**: Password authentication and IP access control (allow/deny lists).
* **Flexible**: Supports file exclusion, attribute preservation, and detailed logging.

### Installation

Build from source:

```bash
# Clone the repository
git clone https://github.com/taurusxin/fastsync.git
cd fastsync

# Build
go build -o fastsync ./cmd/fastsync
```

### Usage

#### 1. Daemon Mode (Server)

Start the server with a configuration file:

```bash
./fastsync -c config.toml
```

See [Configuration](#configuration) for details on `config.toml`.

#### 2. Normal Mode (Client)

Synchronize files between source and target.

**Syntax:**

```bash
fastsync source target [options]
```

* **Source/Target**: Can be a local path or a remote address.
  * Local: `/path/to/dir`
  * Remote: `password@ip:port/instance_name` (Default instance is `default`)
  * *Note*: At least one path must be local.

**Options:**

* `-d`: **Delete**. Delete files in target that are missing in source.
* `-o`: **Overwrite**. Overwrite existing files in target (default is to skip if size/time matches).
* `-s`: **Checksum**. Use content hashing to detect changes (slower but more accurate).
* `-z`: **Compress**. Enable zlib compression during transfer.
* `-a`: **Archive**. Preserve file attributes (permissions, modification time).
* `-t <count>`: **Threads**. Number of concurrent transfer threads (default: 1).
* `-v`: **Verbose**. Print detailed logs during synchronization.

**Examples:**

```bash
# Local to Local
./fastsync ./source ./target -v -a

# Local to Remote (Push)
./fastsync ./source secret@192.168.1.100:7963/backup -z -t 4

# Remote to Local (Pull)
./fastsync secret@192.168.1.100:7963/backup ./restore -d -a
```

### Configuration

A sample configuration file (`fastsync.toml.example`) is provided.

**Global Settings:**

* `address`: Bind address (default "127.0.0.1").
* `port`: Listening port (default 7963).
* `log_level`: Global log level (info, warn, error).
* `log_file`: Path to global log file.

**Instance Settings:**

* `name`: Unique name for the sync module.
* `path`: Local file system path to serve.
* `password`: Authentication password.
* `exclude`: Comma-separated list of glob patterns to ignore.
* `host_allow` / `host_deny`: CIDR IP lists for access control.
* `log_level`: Instance log level.
* `log_file`: Path to instance log file.
