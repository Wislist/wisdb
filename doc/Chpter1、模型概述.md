### 模型概述

五个模块分别为
Data Manager(DM)
Transaction Manager(TM)
Version Manager(VM)
Index Manager(IM)
Table Manager(TBM)

DM是最底层的模块，负责管理数据库文件(DB)以及日志文件(LF).
主要负责：1)对DB进行cache操作，提高对DB的访问顺序；
        2）管理LF，处理回滚啊这些，保证数据库的可恢复性；
        3）分页管理DB，
