# WisDB Architecture

## Overview

WisDB is a lightweight KV-based relational database prototype written in Go. Data flows through a layered architecture: Client → Transporter (TCP) → Parser → Executor → TBM → SM/IM → DM → Disk.

## Module Design

### TM — Transaction Manager

Persists transaction states (active/committed/aborted) in `.xid` file. Assigns monotonically increasing XIDs. Uses mutex for concurrency safety.

### DM — Data Manager

Manages 8KB pages in `.db` file with LRU page cache via Cacher. Implements WAL (Write-Ahead Log) with checksum validation in `.log` file. Crash recovery replays logs: committed transactions are redone, active ones are undone. Validates database integrity via page1 VC (valid check) mechanism.

### SM — Serializability Manager

Implements MVCC: each entry stores XMIN (creator) and XMAX (deleter) for visibility checks. Supports two isolation levels:
- **Read Committed (level 0)**: prevents dirty reads, allows non-repeatable reads
- **Repeatable Read (level 1)**: snapshot isolation, detects version skips

Lock table maintains a directed wait-for graph with DFS-based deadlock detection. Transactions that cause serialization conflicts are automatically aborted.

### IM — Index Manager

B+Tree implementation inspired by boltDB. Each tree has a boot UUID pointing to the root node. Supports concurrent reads/writes with root-node locking. Range queries walk leaf-node sibling chains.

### TBM — Table Manager

Table schemas are stored as a linked list in the database, chained via booter. Each field optionally has a B+Tree index. Translates SQL WHERE conditions into index range scans, merging intervals for AND/OR logic.

### Parser

Custom tokenizer-based SQL parser. Supports DDL (create/drop/show), DML (insert/read/update/delete), and transaction control (begin/commit/abort). WHERE clauses support up to 2 conditions with AND/OR.

### Transporter

Dual protocol support:
- **Wire Protocol v1**: binary framing with magic number (`WISD`), version, request/response types, and pipeline-ready RequestID
- **Hex Transporter**: newline-delimited hex encoding as fallback

### Client

Interactive shell with readline-style input, plus programmatic Go API. Features connection pooling, auto-reconnect on disconnect, and transaction API (`Begin`/`Commit`/`Rollback`).

## Data Flow

```
Client SQL → Wire/Hex Encode → TCP Send
                                        ↓
                              Server Accept → Parser → Executor
                                        ↓
                              TBM (table lookup, index scan)
                                        ↓
                              SM (MVCC visibility, lock acquire)
                                        ↓
                              DM (page cache, WAL logging, disk I/O)
                                        ↓
                              Result → Wire/Hex Encode → TCP Send → Client
```
