<div align="center">

# WisDB

**轻如鸿毛，稳如磐石**

[![GitHub](https://img.shields.io/badge/GitHub-WisDB-blue?logo=GitHub)](https://github.com/Wislist/wisdb)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=Go)](https://go.dev)
[![Cobra](https://img.shields.io/badge/Cobra-1.10%2B-261340?logo=Go)](https://github.com/spf13/cobra)
[![Protocol](https://img.shields.io/badge/Protocol-Wire%20v1-00B0AA?logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxNiIgaGVpZ2h0PSIxNiI+PHBhdGggZD0iTTIgMmg1djJINHY2SDJ6bTEyIDBoNXYySDN2NnoiLz48L3N2Zz4=)](src/main/transporter/wire.go)
[![Build](https://img.shields.io/badge/CI-Go%201.25-0969DA?logo=GitHub%20Actions)](.github/workflows/go.yml)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

WisDB 是一款用 **Go** 从零构建的轻量级 **KV 关系数据库原型**，融合了 **MVCC 事务**、**WAL 崩溃恢复**、**B+Tree 索引**、**死锁检测** 与 **TCP Wire 协议**。它不是任何已有数据库的封装，而是一份完整、自底向上的数据库实现教学样本——每一行存储引擎、每一个并发原语都清晰可读。

**核心价值**：用最少的筹码，造一台能跑事务的数据库；让"数据库是怎么炼成的"不再神秘。

</div>

## 📋 目录

- [🎯 项目概述](#-项目概述)
- [✨ 核心特性](#-核心特性)
- [🌟 技术亮点](#-技术亮点)
- [🏗️ 系统架构](#️-系统架构)
- [🚀 快速开始](#-快速开始)
- [📖 SQL 参考](#-sql-参考)
- [🧩 命令行](#-命令行)
- [📁 项目结构](#-项目结构)
- [🔄 数据流](#-数据流)
- [🧪 测试与性能](#-测试与性能)
- [📚 进阶文档](#-进阶文档)
- [🤝 贡献指南](#-贡献指南)
- [🙏 致谢](#-致谢)

## 🎯 项目概述

WisDB 是一个完整的单机关系型数据库系统，从磁盘管理到 SQL 解析全栈自研，主要特点包括：

- **MVCC 事务**：支持 Read Committed 与 Repeatable Read 两种隔离级别，基于 XMIN/XMAX 版本链实现快照
- **WAL + 恢复**：预写日志 + 校验和，崩溃后自动 redo（已提交）/ undo（未提交）回放
- **B+Tree 并发索引**：借鉴 boltDB 的根节点锁策略，支持并发读写与叶子链范围查询
- **全表扫描回退**：未建索引的字段可走表扫描，配合隐式 rowIndex B+Tree 遍历整表
- **死锁检测**：等待图 + DFS 环检测，冲突事务自动 abort
- **TCP Wire 协议**：二进制帧（`WISD` 魔数）带 RequestID，原生支持 pipeline
- **交互式客户端**：readline 风格原始终端模式，命令历史、箭头键导航
- **Go Driver**：连接池、自动重连、事务 API
- **无 panic**：所有路径返回 error，恢复路径优雅降级

## 项目网页文档
[![zread](https://img.shields.io/badge/Ask_Zread-_.svg?style=flat&color=00b0aa&labelColor=000000&logo=data%3Aimage%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB3aWR0aD0iMTYiIGhlaWdodD0iMTYiIHZpZXdCb3g9IjAgMCAxNiAxNiIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTQuOTYxNTYgMS42MDAxSDIuMjQxNTZDMS44ODgxIDEuNjAwMSAxLjYwMTU2IDEuODg2NjQgMS42MDE1NiAyLjI0MDFWNC45NjAxQzEuNjAxNTYgNS4zMTM1NiAxLjg4ODEgNS42MDAxIDIuMjQxNTYgNS42MDAxSDQuOTYxNTZDNS4zMTUwMiA1LjYwMDEgNS42MDE1NiA1LjMxMzU2IDUuNjAxNTYgNC45NjAxVjIuMjQwMUM1LjYwMTU2IDEuODg2NjQgNS4zMTUwMiAxLjYwMDEgNC45NjE1NiAxLjYwMDFaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00Ljk2MTU2IDEwLjM5OTlIMi4yNDE1NkMxLjg4ODEgMTAuMzk5OSAxLjYwMTU2IDEwLjY4NjQgMS42MDE1NiAxMS4wMzk5VjEzLjc1OTlDMS42MDE1NiAxNC4xMTM0IDEuODg4MSAxNC4zOTk5IDIuMjQxNTYgMTQuMzk5OUg0Ljk2MTU2QzUuMzE1MDIgMTQuMzk5OSA1LjYwMTU2IDE0LjExMzQgNS42MDE1NiAxMy43NTk5VjExLjAzOTlDNS42MDE1NiAxMC42ODY0IDUuMzE1MDIgMTAuMzk5OSA0Ljk2MTU2IDEwLjM5OTlaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik0xMy43NTg0IDEuNjAwMUgxMS4wMzg0QzEwLjY4NSAxLjYwMDEgMTAuMzk4NCAxLjg4NjY0IDEwLjM5ODQgMi4yNDAxVjQuOTYwMUMxMC4zOTg0IDUuMzEzNTYgMTAuNjg1IDUuNjAwMSAxMS4wMzg0IDUuNjAwMUgxMy43NTg0QzE0LjExMTkgNS42MDAxIDE0LjM5ODQgNS4zMTM1NiAxNC4zOTg0IDQuOTYwMVYyLjI0MDFDMTQuMzk4NCAxLjg4NjY0IDE0LjExMTkgMS42MDAxIDEzLjc1ODQgMS42MDAxWiIgZmlsbD0iI2ZmZiIvPgo8cGF0aCBkPSJNNCAxMkwxMiA0TDQgMTJaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00IDEyTDEyIDQiIHN0cm9rZT0iI2ZmZiIgc3Ryb2tlLXdpZHRoPSIxLjUiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIvPgo8L3N2Zz4K&logoColor=ffffff)](https://zread.ai/Wislist/wisdb)

## ✨ 核心特性

### ⚙️ 存储引擎
- **8KB 页式存储**：统一页面管理，LRU 页缓存按需加载
- **页面校验和**：VC（valid check）机制探测磁盘损坏
- **DataItem 编码**：DataSize + Data + 校验位，溢出页支持大对象
- **WAL 日志重放**：崩溃后自动扫描 `.log`，已提交事务重做、未提交事务回滚

### 🔒 事务与并发
- **MVCC 版本链**：每条记录持 XMIN（创建 XID）/ XMAX（删除 XID），按隔离级别做可见性判定
- **两段锁 + 等待图**：共享/排他锁 + DFS 死锁检测，冲突事务自动中止
- **Repeatable Read 快照隔离**：检测版本跳跃并 abort 写者
- **XID 分配**：`.xid` 文件单调递增分配，位图记录每事务状态

### 🌳 索引与查询
- **B+Tree 并发**：根节点锁 + 蟹脚锁协议，读者不阻塞读者
- **范围扫描**：叶子节点兄弟链 `next` 指针顺序遍历
- **区间合并**：WHERE 中 AND/OR 被翻译为索引区间并/交集
- **全表扫描回退**：未建索引的字段走 rowIndex B+Tree 全扫
- **隐式索引**：CREATE TABLE 未显式建索引时自动给首字段建一个

### 💬 SQL 与协议
- **自研 Parser**：递归下降 tokenizer，支持 DDL / DML / 事务 / 聚合
- **ORDER BY / LIMIT / OFFSET**：内存排序与分页
- **聚合函数**：COUNT、SUM、AVG，可配 WHERE 过滤
- **Wire v1 协议**：`WISD` 魔数 + 版本 + RequestID，原生 pipeline
- **Hex 回退协议**：换行分隔十六进制，便于调试
- **交互式 Shell**：原始 tty 模式，回溯历史、行内编辑
- **Go Driver**：连接池、断线重连、`Begin/Commit/Rollback` 事务 API

## 🌟 技术亮点

| 特性 | 技术实现 | 优势 |
|------|----------|------|
| **MVCC 快照隔离** | XMIN/XMAX 版本链 + 快照 XID | 读不阻塞写，写不阻塞读 |
| **WAL + Checksum** | 页内 VC + log 双写 | 普通断电可恢复到一致状态 |
| **B+Tree 蟹脚锁** | root lock + 节点级锁 | 高并发读写不互相阻塞 |
| **DFS 死锁检测** | wait-for graph + 环检测 | 事务自动中止，无需超时 |
| **区间合并索引扫描** | AND/OR → 区间并/交集 | 一条 WHERE 命中多个索引 |
| **隐式 rowIndex 全扫** | 每表自带隐藏 B+Tree | 无索引字段也能查询 |
| **二进制 Wire 协议** | 魔数 + RequestID + 帧 | 原生 pipeline，比文本协议更紧凑 |
| **零 panic 设计** | 全链路 error 返回 | 失败可优雅降级，不崩进程 |
| **Cobra 子命令 CLI** | `serve` / `create` / `--version` | 启停、建库一脚本搞定 |

## 🏗️ 系统架构

```
Client SQL ─→ Wire/Hex Encode ─→ TCP
                                ↓
                          Server Accept
                                ↓
                             Parser ─→ AST
                                ↓
                            Executor
                                ↓
              ┌─────────────────────────────────┐
              │              TBM                  │
              │  表元数据链表  /  字段+索引 B+Tree   │
              │  WHERE → 区间 / 全表扫描分发        │
              └─────────────────────────────────┘
                                ↓
              ┌─────────────────────────────────┐
              │              SM                  │
              │   MVCC 可见性  /  LockTable       │
              │   死锁检测  /  隔离级别            │
              └─────────────────────────────────┘
                                ↓
              ┌─────────────────────────────────┐
              │              IM                  │
              │   B+Tree 并发读写 / 范围扫描       │
              └─────────────────────────────────┘
                                ↓
              ┌─────────────────────────────────┐
              │              DM                  │
              │   8KB 页式  /  LRU 缓存           │
              │   WAL 校验和  /  崩溃恢复          │
              └─────────────────────────────────┘
                                ↓
                             Disk
                                ↓
                  Result ─→ ORDER BY / LIMIT / Aggregates
                                ↓
                       Wire/Hex Encode ─→ TCP ─→ Client
```

## 🚀 快速开始

### 环境要求

- Go 1.25+
- Linux / macOS（Windows 未正式测试）

### 安装步骤

1. **克隆项目**
```bash
git clone https://github.com/Wislist/wisdb.git
cd wisdb
```

2. **构建服务器与客户端**
```bash
go build -o wisdb-server ./src/main/backend/cmd/
go build -o wisdb-client ./src/main/client/
```

3. **创建数据库**
```bash
./wisdb-server create --db-path ./mydb
```
会在 `./mydb/` 下生成 `wisdb.bt`（boot）、`wisdb.db`（数据）、`wisdb.log`（WAL）、`wisdb.xid`（事务状态）四个文件。

4. **启动服务器**
```bash
# 默认：tcp、:3307、内存缓存 64MB
./wisdb-server serve --db-path ./mydb

# 自定义端口与内存
./wisdb-server serve --db-path ./mydb --mem 128MB --addr :4000
```

5. **连接客户端**
```bash
./wisdb-client
# 或自定义
./wisdb-client --addr :4000 --net tcp
```

6. **跑一段 SQL**
```sql
begin
create table user id uint64, name string, age uint32 (index id)
insert into user values 1 alice 20
insert into user values 2 bob 25
insert into user values 3 carol 30
read * from user
read * from user order by age desc limit 3
select count(*) from user
commit
```

### 一键种子数据

仓库自带 `test/seed/seed_data.sql`（320 名员工）：

```bash
go run ./test/seed test/seed/seed_data.sql
```

## 📖 SQL 参考

### 字段类型

| 类型 | 说明 |
|---|---|
| `uint32` | 32 位无符号整数 |
| `uint64` | 64 位无符号整数 |
| `string` | 变长字符串 |

### 事务控制

```sql
begin                                    -- Read Committed（默认）
begin isolation level read committed
begin isolation level repeatable read
commit
abort
```

### DDL

```sql
create table <name> <field> <type> [, ...] (index <field> [<field> ...])
create table user id uint64, name string, age uint32 (index id)
create table order id uint64, uid uint64, amount uint64 (index id uid)

drop table user
show                                      -- 列出所有表
```

### DML

```sql
insert into user values 1 alice 20

read * from user
read id, name from user
read * from user where id = 1
read * from user where age > 18
read * from user where id = 1 and age < 30
read * from user order by age desc limit 10 offset 20

select count(*) from user
select sum(amount) from order where amount > 100
select avg(age) from user where age < 30

update user set name = carol where id = 1
delete from user where id = 1
```

### 运算符

| 运算符 | 说明 |
|---|---|
| `=` / `>` / `<` / `>=` / `<=` / `!=` | 比较 |
| `and` / `or` | 逻辑（最多 2 条件，不可嵌套） |

> 完整语法、边界条件与例子见 [doc/sql-reference.md](doc/sql-reference.md)。

## 🧩 命令行

### 服务器

```
wisdb-server serve  --db-path <path> [--mem 64MB] [--addr :3307] [--net tcp]
wisdb-server create --db-path <path>
wisdb-server --version
```

### 客户端

```
wisdb-client [--addr :3307] [--net tcp]
wisdb-client --version
```

## 📁 项目结构

```
wisdb/
├── src/main/
│   ├── backend/
│   │   ├── cmd/            # cobra 子命令入口：serve / create
│   │   ├── server/         # TCP 服务器 & 执行器
│   │   ├── tm/             # 事务管理器（XID 分配、状态记录）
│   │   ├── dm/             # 数据管理器（页式存储、WAL、恢复）
│   │   │   ├── logger/     # 日志器（校验和、追加写）
│   │   │   ├── pcacher/    # LRU 页缓存
│   │   │   └── pindex/     # 页索引
│   │   ├── sm/             # 串行化（MVCC、锁表、死锁检测）
│   │   │   └── locktable/  # 等待图 + DFS 死锁检测
│   │   ├── im/             # 索引管理器（B+Tree 并发）
│   │   ├── tbm/            # 表管理器（schema、DDL/DML 编译）
│   │   ├── parser/         # SQL 词法/语法解析
│   │   │   └── statement/  # AST 节点
│   │   └── utils/         # 编解码、booter、缓存
│   │       ├── booter/     # 引导 UUID（链表式元数据锚点）
│   │       └── cacher/     # 通用 LRU
│   ├── client/
│   │   ├── client/         # Go driver + 交互式 shell
│   │   └── launcher.go     # cobra 入口
│   └── transporter/        # Wire v1 / Hex 双协议
├── test/
│   ├── integration/        # 端到端集成测试
│   ├── concurrency/        # 并发死锁测试
│   ├── benchmark/          # 性能基准
│   └── seed/               # 种子数据导入
├── doc/
│   ├── architecture.md     # 模块设计与数据流
│   ├── getting-started.md  # 构建运行、CLI、driver、测试指南
│   └── sql-reference.md    # 完整 SQL 语法
├── .github/workflows/      # CI（Go 1.25）
├── go.mod                  # 模块 mydb，go 1.25
└── LICENSE                 # MIT
```

## 🔄 数据流

### 一次查询

```
客户端 SQL → Wire 编码 → TCP 发送
                ↓
        服务器接收 → Parser → AST
                ↓
        Executor 调用 TBM
                ↓
        TBM：表查找 / 索引扫描 vs 全表扫描
                ↓
        SM：MVCC 可见性判定 / 加锁 / 死锁检测
                ↓
        IM：B+Tree 范围读取
                ↓
        DM：页缓存命中？→ 读盘 / WAL 追加
                ↓
        结果 → ORDER BY / LIMIT / 聚合
                ↓
        Wire 编码 → TCP 回送 → 客户端
```

### 崩溃恢复

```
启动 → 检查页1 VC → 校验 DB 完整性
        ↓
扫描 .log → 按事务状态分类
        ↓
已提交 (committed) → redo
未提交 (active)    → undo
        ↓
重写页1 VC → 进入正常服务
```

### 事务生命周期

```
客户端 Begin → TM 分配 XID（.xid 追加）
        ↓
SM 记录事务活跃状态 → 持有共享/排他锁
        ↓
执行 SQL → DM 写页 + 追加 WAL
        ↓
Commit → TM 标记 committed → 释放锁
Abort   → TM 标记 aborted   → 释放锁 + undo
        ↓
等待图在死锁检测周期触发 → 检测到环 → 中止最年轻事务
```

## 🧪 测试与性能

### 测试套件

| 目录 | 覆盖范围 |
|---|---|
| `test/integration/` | 端到端：建表、增删改查、事务、聚合、ORDER BY |
| `test/concurrency/` | 并发事务、死锁触发、隔离级别违反 |
| `test/benchmark/`   | 批量插入、范围扫描、索引 vs 全表扫描时延 |
| `backend/dm/data_manager_test.go` | 页式存储、WAL、恢复 |
| `backend/sm/serializability_manager_test.go` | MVCC、死锁 |
| `backend/im/tree_test.go` | B+Tree 并发读写 |
| `backend/parser/parser_test.go` | SQL 解析 |

### 运行测试

```bash
go test ./...                    # 全量
go test ./src/main/backend/sm -v # 仅串行化模块
go test ./test/concurrency -v    # 并发场景
go test -bench=. ./test/benchmark
```

### 性能参考

> 受硬件、磁盘、页缓存大小影响显著，以下为 macOS / M1 / NVMe 上的粗略数量级：
>
> | 操作 | 吞吐 / 延迟 |
> |---|---|
> | 顺序插入 | ~50k 行/秒（WAL 打开，fsync 关闭） |
> | 索引点查 | < 1ms（热缓存） |
> | 全表扫描 10k 行 | ~50ms |
> | 范围扫描 1k 行 | ~5ms |

**优化思路**：
- 增大 `--mem` 让更多热页留在 LRU 缓存
- 为 WHERE 中常查的字段显式建索引
- 批量插入在单事务内提交，减少 WAL flush 次数

## 📚 进阶文档

| 文档 | 简介 |
|---|---|
| [架构设计](doc/architecture.md) | TM/DM/SM/IM/TBM 分层职责与数据流 |
| [上手指南](doc/getting-started.md) | 构建、运行、CLI、Go driver、测试方法 |
| [SQL 参考](doc/sql-reference.md) | 完整语法：DDL、DML、ORDER BY、LIMIT、聚合 |

## 🤝 贡献指南

欢迎所有形式的贡献——Issue 反馈、文档润色、新特性、Bug 修复都是有效的。

### 开发流程
1. Fork 项目
2. 创建功能分支（`git checkout -b feature/amazing-feature`）
3. 提交更改（`git commit -m 'feat: add amazing feature'`）
4. 推送到分支（`git push origin feature/amazing-feature`）
5. 创建 Pull Request

### 代码规范
- Go 标准格式（`gofmt` / `goimports`）
- 所有路径返回 `error`，不在正常路径 `panic`
- 包名小写、单词、无下划线
- 提交信息前缀：`feat:` / `fix:` / `refactor:` / `docs:` / `test:`
- 新特性需附带测试

## 🙏 致谢

WisDB 的设计受到以下项目和资料的启发：

- [boltdb](https://github.com/boltdb/bolt) — B+Tree 蟹脚锁与 mvcc 设计参考
- *Database System Concepts*（Silberschatz et al.） — 事务、恢复、并发控制理论
- *Designing Data-Intensive Applications*（Martin Kleppmann） — MVCC 与复制原理
- [cobra](https://github.com/spf13/cobra) — CLI 子命令框架
- [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) — 原始终端模式

---

<div align="center">

**[🔝 返回顶部](#wisdb)**

Made with ❤️ by [Wislist](https://github.com/Wislist)

</div>