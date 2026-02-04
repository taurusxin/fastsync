# fastsync

⚡ **fastsync** 是一个跨平台、高性能的文件同步工具，灵感来自 rsync，
但专为 **现代系统和更简单的使用体验** 而设计。

它原生支持 **Linux / macOS / Windows**，无需 rsync、WSL 或任何兼容层，
适合在多平台环境中进行稳定、高效的文件同步。

---

## 为什么选择 fastsync？

rsync 非常强大，但在实际使用中也存在一些痛点：

- Windows 环境下部署和使用成本较高
- 参数体系复杂，学习和维护成本大
- 服务端配置不够直观

**fastsync 的设计目标是：**

- 🚀 **跨平台一致体验**：一次构建，到处运行
- 🧩 **模块化设计**：服务端实例清晰、职责明确
- ⚙️ **简单配置**：使用直观的 TOML 配置文件
- 🔐 **内置安全机制**：密码认证 + IP 访问控制

---

## 功能特性

- **跨平台支持**
  - Linux / macOS / Windows
- **双运行模式**
  - **守护模式（Daemon）**：作为文件同步服务端长期运行
  - **普通模式（CLI）**：用于本地或远程的一次性同步
- **高效传输**
  - 增量同步
  - 可选压缩传输
- **安全性**
  - 密码认证
  - IP 白名单 / 黑名单控制
- **灵活可控**
  - 文件排除规则
  - 文件属性保留
  - 支持实例级日志配置

---

## 安装方式

从源码编译：

```bash
git clone https://github.com/taurusxin/fastsync.git
cd fastsync
go build -o fastsync ./cmd/fastsync
```

### Docker 部署

你也可以使用 Docker 来运行 Fastsync。

1. **准备配置文件**：
    创建一个 `config.toml` 文件。你可以参考 `config.toml.example`。确保配置文件中的路径指向容器内的挂载点（如 `/data`）。

    ```toml
    # Docker 配置文件示例片段
    [[instances]]
    path = "/data"  # 映射到容器内的路径
    ```

2. **运行容器**：

    ```bash
    docker run -d \
      --name fastsync \
      -p 7963:7963 \
      -v $(pwd)/config.toml:/config/config.toml \
      -v $(pwd)/data:/data \
      taurusxin/fastsync:latest
    ```

## 使用方法

### 1. 守护模式 (服务端)

如果你不打算用 Docker 部署，也可以使用配置文件来直接启动服务：

```bash
./fastsync -c config.toml
```

配置文件详情请参考 [配置说明](#配置说明)。

### 2. 普通模式 (客户端)

在源目录和目标目录之间同步文件。

**语法：**

```bash
fastsync source target [options]
```

- **Source/Target**：可以是本地路径或远程地址。
  - 本地：`/path/to/dir`
  - 远程：`password@ip:port/instance_name` (实例名默认为 `default`)
  - *注意*：源和目标中至少有一个必须是本地路径。

**选项：**

- `-d`: **删除 (Delete)**。如果源中文件已删除，则同步删除目标中的文件。
- `-o`: **覆盖 (Overwrite)**。遇到同名文件直接覆盖（默认行为是如果大小/时间匹配则跳过）。
- `-s`: **哈希校验 (Checksum)**。使用内容哈希检测文件变化（较慢但更准确）。
- `-z`: **压缩 (Compress)**。传输时启用 zlib 压缩。
- `-a`: **归档 (Archive)**。保留文件属性（权限、修改时间等）。
- `-v`: **详细 (Verbose)**。同步时输出详细日志。

**示例：**

```bash
# 本地同步到本地
./fastsync ./source ./target -v -a

# 本地同步到远程 (推模式)
./fastsync ./source secret@192.168.1.100:7963/backup -z

# 远程同步到本地 (拉模式)
./fastsync secret@192.168.1.100:7963/backup ./restore -d -a
```

## 配置说明

项目根目录下提供了示例配置文件 `fastsync.toml.example`。

**全局配置：**

- `address`: 绑定地址 (默认 "127.0.0.1")。
- `port`: 监听端口 (默认 7963)。
- `log_level`: 全局日志等级 (info, warn, error)。
- `log_file`: 全局日志文件路径。

**实例配置：**

- `name`: 同步模块的唯一名称。
- `path`: 服务端提供的本地文件路径。
- `password`: 认证密码。
- `exclude`: 逗号分隔的忽略文件模式列表。
- `host_allow` / `host_deny`: 允许/拒绝连接的 IP CIDR 列表。
- `log_level`: 实例日志等级。
- `log_file`: 实例日志文件路径。

## 计划功能

- [ ] 配置文件热重载
- [ ] 文件加密传输
- [ ] 增量传输
- [ ] 断点续传
- [ ] 大文件多点分片传输
