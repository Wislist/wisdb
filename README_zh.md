# WisDB

基于 Go 语言实现的轻量级 KV 型关系数据库原型，支持事务、MVCC、WAL 恢复、B+Tree 索引和 TCP 客户端访问。

## 快速开始

```bash
# 编译
go build -o wisdb-server ./src/main/backend/cmd/
go build -o wisdb-client ./src/main/client/

# 创建数据库
./wisdb-server -create ./mydb

# 启动服务端（端口 3307，缓存 64MB）
./wisdb-server -open ./mydb

# 连接客户端
./wisdb-client
```

```sql
begin
create table user id uint64, name string, age uint32 (index id)
insert into user values 1 alice 20
insert into user values 2 bob 25
read * from user
commit
```

## 文档

| 文档 | 说明 |
|---|---|
| [架构设计](doc/architecture.md) | 模块设计与数据流（英文） |
| [入门指南](doc/getting-started.md) | 编译、运行、驱动、测试（英文） |
| [SQL 参考](doc/sql-reference.md) | 完整语法与示例（英文） |

English：[README.md](README.md)

## 功能特性

- **MVCC** — 读已提交 & 可重复读隔离级别
- **WAL + 崩溃恢复** — redo/undo 日志回放保证数据安全
- **B+Tree 索引** — 支持并发读写、范围查询
- **死锁检测** — 基于 DFS 的等待图环检测
- **TCP 协议** — 自定义 Wire 协议，支持 pipeline
- **Go Driver** — 连接池、事务、自动重连

## 项目结构

```
mydb-go/
├── src/main/backend/
│   ├── cmd/          # 服务端入口
│   ├── server/       # TCP 服务端 & 执行器
│   ├── tm/           # 事务管理器
│   ├── dm/           # 数据管理器（页缓存、WAL、恢复）
│   ├── sm/           # 可串行化管理器（MVCC、锁表）
│   ├── im/           # 索引管理器（B+Tree）
│   ├── tbm/          # 表管理器（表结构、DDL/DML）
│   └── parser/       # SQL 解析器
├── src/main/client/  # 交互式客户端
├── src/main/transporter/  # Wire 协议编解码
├── test/             # 集成测试、并发测试、基准测试
└── doc/              # 文档
```

## 开源协议

[MIT](LICENSE)
