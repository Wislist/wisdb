# Getting Started

## Requirements

- Go 1.23+

## Build

```bash
# Server binary
go build -o wisdb-server ./src/main/backend/cmd/

# Client binary
go build -o wisdb-client ./src/main/client/

# Seed tool (batch SQL executor)
go build -o wisdb-seed ./test/seed/
```

## Create Database

First use creates four files (`.db`, `.log`, `.xid`, `.bt`) at the given path:

```bash
./wisdb-server -create /path/to/mydb
```

> Must use `-create` before first `-open`. Re-running `-create` on an existing database will panic.

## Start Server

```bash
# Default: port 3307, 64MB memory cache
./wisdb-server -open /path/to/mydb

# Custom memory (supports KB, MB, GB)
./wisdb-server -open /path/to/mydb -mem 128MB
```

Press `Ctrl+C` for graceful shutdown.

## Connect Client

```bash
./wisdb-client
```

Connects to `localhost:3307` by default. Enters interactive SQL shell — type SQL statements directly.

## Batch Import

```bash
./wisdb-seed ./test/seed/seed_data.sql
```

Executes SQL statements line-by-line from a file.

## Go Driver

```go
import "wisdb-go-driver"

pool, err := wisdb.NewPool("localhost:3307", wisdb.DefaultPoolOptions())
conn, err := pool.Get()
defer pool.Put(conn)

result, err := conn.Execute("read * from user where id = 1")
```

### Pool Configuration

```go
opts := wisdb.PoolOptions{
    MaxConns:    10,
    MinConns:    2,
    DialTimeout: 10 * time.Second,
    IdleTimeout: 5 * time.Minute,
}
pool, err := wisdb.NewPool("localhost:3307", opts)
```

### Transactions via Driver

```go
tx, err := conn.Begin(wisdb.ReadCommitted)
tx.Execute("insert into user values 1 alice 20")
tx.Commit()
// or tx.Rollback()
```

### Isolation Levels

| Constant | Description |
|---|---|
| `wisdb.ReadCommitted` | Read Committed (default) |
| `wisdb.RepeatableRead` | Repeatable Read |

## Running Tests

```bash
# Unit tests
go test ./src/...

# Integration test (full client-server round trip)
go test ./test/integration/

# Concurrency test (50/100 parallel workers)
go test ./test/concurrency/

# Benchmark (generates results.csv)
go test ./test/benchmark/ -v -timeout 300s
```

## Database Files

| Extension | Purpose |
|---|---|
| `.db` | Page-based data storage (8KB pages) |
| `.log` | Write-Ahead Log with checksums |
| `.xid` | Transaction ID counter and status records |
| `.bt` | Table boot metadata (first table UUID) |
