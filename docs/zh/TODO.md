# GhostLink TODO

## Phase 1 — 核心功能（MVP）

### 1.1 项目初始化
- [x] 初始化 Go Module（`go mod init`）
- [x] 搭建项目目录结构（`cmd/`, `internal/`, `pkg/`）
- [x] 引入 CLI 框架（cobra）
- [x] 配置 main 入口及根命令
- [x] 添加 Makefile（build / test / lint）

### 1.2 钱包管理（wallet）
- [x] 实现 Solana Ed25519 密钥对生成
- [x] 支持导入已有私钥（Base58 格式）
- [x] 支持通过助记词导入/导出
- [x] 本地加密存储私钥（AES-256，密码保护）
- [x] 实现 `wallet create` 子命令
- [x] 实现 `wallet balance` 子命令（查询 SOL 余额）
- [x] 钱包文件默认存储路径 `~/.ghostlink/`

### 1.3 加密模块（crypto）
- [x] 实现基于 Solana 公钥的非对称加密（NaCl box / X25519）
- [x] 实现消息加密：发送方使用收件方公钥加密
- [x] 实现消息解密：收件方使用自己私钥解密
- [x] 验证加密后数据不超过 512 字节
- [x] 编写加解密单元测试

### 1.4 Solana 客户端（solana）
- [x] 封装 Solana JSON-RPC 客户端
- [x] 连接 Solana devnet
- [x] 实现 Memo Program 交易构造
- [x] 实现交易签名与提交
- [x] 实现交易查询（按地址获取历史交易）
- [x] 实现 Memo 指令解析（从交易中提取 Memo 数据）

### 1.5 发送消息（send）
- [x] 实现 `send` 子命令
- [x] 参数解析：`--to`（收件方地址）、`--message / -m`（消息内容）
- [x] 流程：验证消息长度 → 加密 → 构造 Memo 交易 → 签名 → 提交
- [x] 返回交易签名（tx hash）作为发送凭证
- [x] 错误处理：消息过长、余额不足、网络异常

### 1.6 接收消息（receive）
- [x] 实现 `receive` 子命令
- [x] 参数解析：`--inbox`、`--limit`、`--since`
- [x] 流程：查询交易 → 过滤 Memo 交易 → 解密 → 格式化输出
- [x] 展示信息：发送方地址、时间戳、解密后的消息内容
- [x] 处理解密失败的情况（非本人消息静默跳过）

## Phase 2 — 隐私增强

### 2.1 隐形收件箱（inbox）
- [x] 本地生成收件箱地址（派生密钥对）
- [x] 实现 `inbox create` 子命令
- [x] 支持多个收件箱管理（列表、切换、删除）
- [x] 收件箱元数据本地加密存储
- [ ] 实现基于 Address Lookup Table (ALT) 的链上隐形收件箱
- [ ] 创建链上 Address Lookup Table

### 2.2 二维码分享
- [x] 实现 `inbox share` 子命令
- [x] 生成包含收件箱地址的二维码
- [x] 支持终端字符画输出
- [x] 支持导出为 PNG 图片
- [x] 扫码端解析收件箱地址

### 2.3 Tor 代理集成
- [x] 封装 SOCKS5 代理 HTTP 客户端
- [x] 支持 `--tor` 全局标志启用 Tor 代理
- [x] 默认代理地址 `127.0.0.1:9050`，支持自定义
- [x] 所有 Solana RPC 请求通过 Tor 转发
- [x] Tor 连接状态检测与错误提示

## Phase 3 — 完善体验

### 3.1 配置文件
- [x] 支持 JSON 配置文件（`~/.ghostlink/config.json`）
- [x] 可配置项：默认钱包、RPC 节点地址、Tor 开关、代理地址
- [x] 命令行参数优先于配置文件

### 3.2 跨平台发布
- [x] 配置 Makefile 交叉编译
- [x] 支持 Linux (amd64/arm64)
- [x] 支持 macOS (amd64/arm64)
- [x] 支持 Windows (amd64)
- [x] 生成单二进制文件，无外部依赖

### 3.3 错误处理与用户提示
- [x] 统一错误信息格式
- [x] 完善 `--help` 文档（每个子命令）
- [ ] 友好的用户交互提示（进度条、确认对话）
- [ ] 添加 `--verbose / -v` 调试模式

### 3.4 文档与示例
- [ ] 编写使用文档（安装、快速开始、命令参考）
- [ ] 添加端到端使用示例（发送 → 接收完整流程）
- [ ] 安全注意事项说明
