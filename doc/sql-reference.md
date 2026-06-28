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

Supports `*` (all fields) or comma-separated field names. WHERE requires indexed fields, max 2 conditions.

```sql
read * from user
read id, name from user
read * from user where id = 1
read * from user where age > 18
read * from user where id = 1 and age < 30
read * from user where id = 1 or id = 2
```

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
read * from user
read * from user where id = 1
update user set name = dave where id = 2
delete from user where id = 3
commit
show
```

## Important Notes

- WHERE fields **must** be indexed; querying by unindexed fields returns an error
- UPDATE modifies exactly **one** field per statement
- WHERE supports at most **2** conditions; no nesting
- String values are **unquoted**, separated by whitespace
- Database file paths must **not** contain spaces
