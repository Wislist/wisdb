# mydb-go / WisDB

基于 Go 语言实现的轻量级 KV 型关系数据库原型，支持事务、MVCC、WAL 恢复、B+Tree 索引和 TCP 客户端访问。

## 项目结构

```
mydb-go/
├── src/main/backend/
│   ├── cmd/              # 服务端启动入口
│   ├── server/           # TCP 服务端 & 执行器
│   ├── tm/               # Transaction Manager：事务状态持久化
│   ├── dm/               # Data Manager：页缓存、WAL、恢复
│   ├── sm/               # Serializability Manager：MVCC、锁表、死锁检测
│   ├── im/               # Index Manager：B+Tree 索引
│   ├── tbm/              # Table Manager：表结构、DDL/DML
│   └── parser/           # SQL 解析器
├── src/main/client/      # 交互式客户端启动入口
├── src/main/transporter/ # Wire Protocol 编解码
├── driver/go/wisdb/      # 独立 Go Driver
└── test/                 # 集成测试、并发测试、基准测试
```

## 快速开始

### 环境要求

- Go 1.23+

### 编译

```bash
# 编译服务端
go build -o wisdb-server ./src/main/backend/cmd/

# 编译客户端
go build -o wisdb-client ./src/main/client/
```

### 创建数据库

```bash
./wisdb-server -create /path/to/mydb
```

首次使用必须先 `-create`，会在指定路径生成 `.db`、`.log`、`.xid`、`.bt` 四个文件。

### 启动服务端

```bash
# 默认监听 :8080，内存缓存 64MB
./wisdb-server -open /path/to/mydb

# 指定内存大小
./wisdb-server -open /path/to/mydb -mem 128MB
```

`-mem` 支持 `KB`、`MB`、`GB` 单位，默认 64MB。

### 启动客户端

```bash
./wisdb-client
```

客户端默认连接 `localhost:8080`，进入交互式 Shell 后可直接输入 SQL 语句。

---

## SQL 语法参考

### 字段类型

| 类型     | 说明            |
| -------- | --------------- |
| `uint32` | 32 位无符号整数 |
| `uint64` | 64 位无符号整数 |
| `string` | 字符串          |

### 比较运算符

| 运算符 | 说明 |
| ------ | ---- |
| `=`    | 等于 |
| `>`    | 大于 |
| `<`    | 小于 |

### 逻辑运算符

| 运算符 | 说明   |
| ------ | ------ |
| `and`  | 逻辑与 |
| `or`   | 逻辑或 |

> `where` 子句最多支持两个条件表达式，通过 `and` / `or` 连接。

---

### 事务控制

#### begin

开启一个事务，默认隔离级别为 Read Committed。

```sql
begin
begin isolation level read committed
begin isolation level repeatable read
```

#### commit

提交当前事务。

```sql
commit
```

#### abort

回滚当前事务。

```sql
abort
```

---

### DDL

#### create table

创建表，必须指定至少一个索引字段。

```
create table <表名> <字段名> <类型> [, <字段名> <类型> ...] (index <字段名> [<字段名> ...])
```

```sql
create table user id uint64, name string, age uint32 (index id)
create table order id uint64, uid uint64, amount uint64 (index id uid)
```

- 字段名和表名只能包含字母、数字和下划线，不能是单个非字母字符。
- 每张表至少需要一个索引字段，未建索引的字段不支持 `where` 条件查询。

#### drop table

删除表。

```
drop table <表名>
```

```sql
drop table user
```

#### show

列出当前可见的所有表。

```sql
show
```

---

### DML

#### insert

向表中插入一条记录，值按字段定义顺序依次提供。

```
insert into <表名> values <值1> <值2> ...
```

```sql
insert into user values 1 alice 20
insert into user values 2 bob 25
```

- 字符串值不需要引号，直接写字面量。
- 值的顺序必须与 `create table` 时字段定义的顺序一致。

#### read

查询记录，支持查询全部字段或指定字段，支持 `where` 条件。

```
read (* | <字段名> [, <字段名> ...]) from <表名> [where <条件>]
```

```sql
-- 查询全部字段
read * from user

-- 查询指定字段
read id, name from user

-- 带条件查询（字段必须有索引）
read * from user where id = 1
read * from user where age > 18
read * from user where id = 1 and age < 30
read * from user where id = 1 or id = 2
```

> `where` 条件中的字段必须是建立了索引的字段，否则返回错误。

#### update

更新表中满足条件的记录的某个字段值，每次只能更新一个字段。

```
update <表名> set <字段名> = <新值> [where <条件>]
```

```sql
update user set name = carol where id = 1
update user set age = 30 where id = 2
```

#### delete

删除满足条件的记录。

```
delete from <表名> where <条件>
```

```sql
delete from user where id = 1
delete from user where age < 18
```

---

## 完整示例

```sql
-- 开启事务
begin

-- 建表
create table user id uint64, name string, age uint32 (index id)

-- 插入数据
insert into user values 1 alice 20
insert into user values 2 bob 25
insert into user values 3 carol 30

-- 查询
read * from user
read * from user where id = 1
read id, name from user where age > 20

-- 更新
update user set name = dave where id = 2

-- 删除
delete from user where id = 3

-- 提交
commit

-- 查看所有表
show
```

---

## Go Driver

项目提供独立的 Go Driver，位于 `driver/go/wisdb/`，可作为独立模块引入。

```go
import "wisdb-go-driver"

// 连接池
pool, err := wisdb.NewPool("localhost:8080", wisdb.DefaultPoolOptions())
conn, err := pool.Get()
defer pool.Put(conn)

// 执行查询
result, err := conn.Execute("read * from user where id = 1")

// 事务
tx, err := conn.Begin(wisdb.ReadCommitted)
tx.Execute("insert into user values 4 eve 22")
tx.Commit()
// 或 tx.Rollback()
```

### 连接池配置

```go
opts := wisdb.PoolOptions{
    MaxConns:    10,
    MinConns:    2,
    DialTimeout: 10 * time.Second,
    IdleTimeout: 5 * time.Minute,
}
pool, err := wisdb.NewPool("localhost:8080", opts)
```

### 隔离级别

| 常量                   | 说明             |
| ---------------------- | ---------------- |
| `wisdb.ReadCommitted`  | 读已提交（默认） |
| `wisdb.RepeatableRead` | 可重复读         |

---

## 运行测试

```bash
# 全量测试
go test ./...

# 并发测试
go test ./test/concurrency/

# 基准测试（输出 results.csv）
go test ./test/benchmark/ -v -timeout 300s

# Wire Protocol 测试
go test ./src/main/transporter/

# Go Driver 测试
cd driver/go/wisdb && go test ./...
```

---

## 注意事项

- `where` 条件中的字段必须建立了索引，否则查询会报错。
- `update` 每次只能修改一个字段。
- `where` 最多支持两个条件，通过 `and` 或 `or` 连接，不支持嵌套。
- 字符串值不需要引号，token 以空白符分隔。
- 数据库文件路径不能包含空格。
