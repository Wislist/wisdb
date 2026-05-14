-- ============================================================
-- 前置：建表（如果已有 employee 表可跳过这一段）
-- ============================================================
begin
create table account id uint64, name string, balance uint64 (index id)
commit


-- ============================================================
-- 1. 基本事务：提交
-- ============================================================
begin
insert into account values 1 alice 10000
insert into account values 2 bob 5000
insert into account values 3 carol 8000
commit

-- 验证数据已提交
read * from account


-- ============================================================
-- 2. 基本事务：回滚（abort）
--    插入后 abort，数据不应出现
-- ============================================================
begin
insert into account values 4 dave 9999
abort

-- 验证 id=4 不存在（应返回空）
read * from account where id = 4


-- ============================================================
-- 3. 转账模拟：两步更新在同一事务内
--    alice 转 2000 给 bob
-- ============================================================
begin
update account set balance = 8000 where id = 1
update account set balance = 7000 where id = 2
commit

-- 验证转账结果
read * from account where id = 1
read * from account where id = 2


-- ============================================================
-- 4. 转账失败回滚：更新后发现异常，abort
-- ============================================================
begin
update account set balance = 6000 where id = 1
update account set balance = 9000 where id = 2
abort

-- 验证余额未变（仍是上一次提交的值）
read * from account where id = 1
read * from account where id = 2


-- ============================================================
-- 5. Read Committed 隔离级别
--    默认级别，每次读取都能看到已提交的最新数据
-- ============================================================
begin isolation level read committed
read * from account where id = 1
update account set balance = 1000 where id = 3
commit

read * from account where id = 3


-- ============================================================
-- 6. Repeatable Read 隔离级别
--    事务内多次读取同一行，结果一致
-- ============================================================
begin isolation level repeatable read
read * from account where id = 1
read * from account where id = 2
commit


-- ============================================================
-- 7. 批量插入后统一提交
-- ============================================================
begin
insert into account values 10 user10 1000
insert into account values 11 user11 2000
insert into account values 12 user12 3000
insert into account values 13 user13 4000
insert into account values 14 user14 5000
commit

read * from account where id > 9


-- ============================================================
-- 8. 批量插入后回滚
-- ============================================================
begin
insert into account values 20 ghost1 100
insert into account values 21 ghost2 200
insert into account values 22 ghost3 300
abort

-- 验证这些记录不存在
read * from account where id = 20
read * from account where id = 21


-- ============================================================
-- 9. 事务内删除 + 提交
-- ============================================================
begin
delete from account where id = 14
commit

-- 验证已删除
read * from account where id = 14


-- ============================================================
-- 10. 事务内删除 + 回滚（数据应保留）
-- ============================================================
begin
delete from account where id = 13
abort

-- 验证数据仍存在
read * from account where id = 13


-- ============================================================
-- 最终全表查询，确认数据状态
-- ============================================================
read * from account
