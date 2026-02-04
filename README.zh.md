# Fastsync

**Fastsync** 是一个跨平台、高性能的文件同步工具，旨在成为 rsync 的现代替代品。它支持本地和远程文件同步，拥有模块化的设计和简单的配置。

## 特性

* **跨平台**：支持 Linux, macOS 和 Windows。
   **双模式**：
  * **守护模式**：作为服务器运行，通过 TOML 文件配置。
  * **普通模式**：作为命令行工具运行，用于临时同步（本地-本地 或 本地-远程）。
* **高效**：支持增量同步和数据压缩。
* **安全**：支持密码认证和 IP 访问控制（白名单/黑名单）。
* **灵活**：支持文件排除、属性保留和详细日志记录。

### 安装

从源码编译：

```bash
# 克隆仓库
git clone https://github.com/yourusername/fastsync.git
cd fastsync

# 编译
go build -o fastsync ./cmd/fastsync
```

### 使用方法

#### 1. 守护模式 (服务端)

指定配置文件启动服务器：

```bash
./fastsync -c config.toml
```

配置文件详情请参考 [配置说明](#配置说明)。

#### 2. 普通模式 (客户端)

在源目录和目标目录之间同步文件。

**语法：**

```bash
fastsync source target [options]
```

* **Source/Target**：可以是本地路径或远程地址。
  * 本地：`/path/to/dir`
  * 远程：`password@ip:port/instance_name` (实例名默认为 `default`)
  * *注意*：源和目标中至少有一个必须是本地路径。

**选项：**

* `-d`: **删除 (Delete)**。如果源中文件已删除，则同步删除目标中的文件。
* `-o`: **覆盖 (Overwrite)**。遇到同名文件直接覆盖（默认行为是如果大小/时间匹配则跳过）。
* `-s`: **哈希校验 (Checksum)**。使用内容哈希检测文件变化（较慢但更准确）。
* `-z`: **压缩 (Compress)**。传输时启用 zlib 压缩。
* `-a`: **归档 (Archive)**。保留文件属性（权限、修改时间等）。
* `-v`: **详细 (Verbose)**。同步时输出详细日志。

**示例：**

```bash
# 本地同步到本地
./fastsync ./source ./target -v -a

# 本地同步到远程 (推模式)
./fastsync ./source secret@192.168.1.100:7963/backup -z

# 远程同步到本地 (拉模式)
./fastsync secret@192.168.1.100:7963/backup ./restore -d -a
```

### 配置说明

项目根目录下提供了示例配置文件 `fastsync.toml.example`。

**全局配置：**

* `address`: 绑定地址 (默认 "127.0.0.1")。
* `port`: 监听端口 (默认 7963)。
* `log_level`: 全局日志等级 (info, warn, error)。
* `log_file`: 全局日志文件路径。

**实例配置：**

* `name`: 同步模块的唯一名称。
* `path`: 服务端提供的本地文件路径。
* `password`: 认证密码。
* `exclude`: 逗号分隔的忽略文件模式列表。
* `host_allow` / `host_deny`: 允许/拒绝连接的 IP CIDR 列表。
* `log_level`: 实例日志等级。
* `log_file`: 实例日志文件路径。
