# SQL Reference

## Field Types

| Type | Description |
|---|---|
| `uint32` | 32-bit unsigned integer |
| `uint64` | 64-bit unsigned integer |
| `string` | Variable-length string |

## Transaction Control

```sql
begin                                    -- Read Committed (default)
begin isolation level read committed
begin isolation level repeatable read
commit
abort
```

## DDL

### CREATE TABLE

Table and field names: letters, digits, underscores only. Must define at least one indexed field.

```sql
create table <name> <field> <type> [, ...] (index <field> [<field> ...])
```

```sql
create table user id uint64, name string, age uint32 (index id)
create table order id uint64, uid uint64, amount uint64 (index id uid)
```

### DROP TABLE

```sql
drop table user
```

### SHOW

Lists all visible tables with their field definitions.

```sql
show
```

## DML

### INSERT

Values in field-definition order. Strings need no quotes.

```sql
insert into user values 1 alice 20
```

### READ

Supports `*` (all fields), comma-separated field names, or aggregate functions. WHERE conditions use `=`, `>`, `<` with `and`/`or` (max 2 conditions). Unindexed fields fall back to full table scan.

```sql
read * from user
read id, name from user
read * from user where id = 1
read * from user where age > 18
read * from user where id = 1 and age < 30
read * from user where id = 1 or id = 2
```

#### ORDER BY

Sort results by any field. Default ascending; use `desc` for descending.

```sql
read * from user order by age
read * from user order by age desc
read * from user where age > 18 order by name
```

#### LIMIT / OFFSET

Limit the number of returned rows, optionally with an offset.

```sql
read * from user limit 5
read * from user limit 10 offset 20
read * from user order by age desc limit 3
```

#### Aggregate Functions

| Function | Description |
|---|---|
| `count(*)` | Total number of rows |
| `count(field)` | Alias for count(*) |
| `sum(field)` | Sum of numeric field values |
| `avg(field)` | Average of numeric field values |

Aggregates can be combined with WHERE and ORDER BY for filtered, sorted aggregation:

```sql
select count(*) from user
select sum(amount) from order where amount > 100
select avg(age) from user where age < 30 order by age
```

> Note: When using aggregates, `select` is accepted as an alias for `read`.

### UPDATE

Updates one field per statement.

```sql
update user set name = carol where id = 1
```

### DELETE

```sql
delete from user where id = 1
delete from user where age < 18
```

## Operators

| Operator | Description |
|---|---|
| `=` | Equal |
| `>` | Greater than |
| `<` | Less than |
| `and` | Logical AND |
| `or` | Logical OR |

## Complete Example

```sql
begin
create table user id uint64, name string, age uint32 (index id)
insert into user values 1 alice 20
insert into user values 2 bob 25
insert into user values 3 carol 30
insert into user values 4 dave 22
insert into user values 5 eve 27
read * from user
read * from user where id = 1
read * from user where age > 20 order by age
read * from user order by age desc limit 3
select count(*) from user
select avg(age) from user
update user set name = dave2 where id = 2
delete from user where id = 3
commit
show
```

## Important Notes

- **Indexed fields**: WHERE on indexed fields uses B+Tree range scan (fast)
- **Unindexed fields**: WHERE on unindexed fields uses full table scan (slower, but works)
- WHERE supports at most **2** conditions; no nesting
- UPDATE modifies exactly **one** field per statement
- String values are **unquoted**, separated by whitespace
- Database file paths must **not** contain spaces
