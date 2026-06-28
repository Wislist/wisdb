# WisDB

A lightweight KV-based relational database prototype written in Go, featuring transactions, MVCC, WAL recovery, B+Tree indexing, and a TCP client interface.

## Quick Start

```bash
# Build
go build -o wisdb-server ./src/main/backend/cmd/
go build -o wisdb-client ./src/main/client/

# Create database
./wisdb-server -create ./mydb

# Start server
./wisdb-server -open ./mydb                   # defaults: tcp, :3307, 64MB
./wisdb-server -open ./mydb -mem 128MB -addr :4000

# Connect
./wisdb-client
```

```sql
begin
create table user id uint64, name string, age uint32 (index id)
insert into user values 1 alice 20
insert into user values 2 bob 25
read * from user
read * from user order by age desc limit 3
select count(*) from user
commit
```

## Documentation

| Document | Description |
|---|---|
| [Architecture](doc/architecture.md) | Module design and data flow |
| [Getting Started](doc/getting-started.md) | Build, run, CLI flags, driver usage, tests |
| [SQL Reference](doc/sql-reference.md) | Full syntax: DDL, DML, ORDER BY, LIMIT, aggregates |

中文文档：[README_zh.md](README_zh.md)

## Features

- **MVCC** — Read Committed & Repeatable Read isolation levels
- **WAL + Recovery** — Crash-safe with redo/undo log replay
- **B+Tree Index** — Concurrent reads/writes, range queries
- **Full Scan** — Queries on unindexed fields via table scan
- **ORDER BY / LIMIT** — Sort results, paginate with OFFSET
- **Aggregates** — COUNT, SUM, AVG with WHERE filtering
- **Deadlock Detection** — DFS cycle detection in wait-for graph
- **TCP Protocol** — Custom wire protocol with pipeline support
- **Go Driver** — Connection pool, transactions, auto-reconnect
- **Error Handling** — All layers return errors, no panics in normal operation

## Project Structure

```
mydb-go/
├── src/main/backend/
│   ├── cmd/          # Server entry point
│   ├── server/       # TCP server & executor
│   ├── tm/           # Transaction Manager
│   ├── dm/           # Data Manager (pages, WAL, recovery)
│   ├── sm/           # Serializability Manager (MVCC, locks)
│   ├── im/           # Index Manager (B+Tree)
│   ├── tbm/          # Table Manager (schemas, DDL/DML)
│   └── parser/       # SQL parser
├── src/main/client/  # Interactive client
├── src/main/transporter/  # Wire protocol
├── test/             # Integration, concurrency, benchmark tests
└── doc/              # Documentation
```

## License

[MIT](LICENSE)
