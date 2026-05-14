# 基于 mydb-go 项目的本科毕业论文定稿规划（Strategist 交接包）

> 适用项目：`/Users/wislist/Desktop/worksplace/mydb-go`  
> 学校模板/规范来源：  
> - `北京理工大学珠海学院本科生毕业设计（论文）模板（2022年9月）.docx`  
> - `北京理工大学珠海学院本科生毕业设计（论文）书写规范及打印装订要求(2022年9月).doc`  
> 参考案例：`论文案例1- 仅限群内参考-勿外传.pdf`（当前环境无法抽取 PDF 文本，以下仅把它当作“排版与表达风格参考”，不作为事实来源）

本文件的目标是把论文内容严格锚定到 `mydb-go` 的真实实现，给后续写作/定稿（`academic-paper-composer`）提供可执行的证据映射、重写范围与图表/截图清单，避免“看起来像论文，但与代码不一致”的风险。

---

## 1. 约束与格式要点（从学校规范中抽取）

1. **结构顺序（必须包含）**：封面、原创性声明、使用授权声明、中英文摘要与关键词、目录、正文、结论、参考文献、附录（可选）、致谢。
1. **页眉/页码**：
   - 除封面外各页均添加页眉（示例内容：`北京理工大学珠海学院20XX届本科生毕业设计（论文）`）。
   - 声明/摘要/目录等用罗马数字，正文用阿拉伯数字（以学校规范为准）。
1. **目录**：自动目录；仅显示到三级标题。
1. **图表编号与标题**：
   - 图/表按章编号（例：`图1-1`、`表2-1`）。
   - 图注在图下，表题在表上；按规范字体字号居中。
1. **参考文献**：按 `GB/T 7714—2015`。
1. **模板文本框/提示语**：属于“过程性说明”，定稿必须全部删除（如“阅后删除此文本框”等）。

---

## 2. 项目证据快速盘点（可用于论文叙述的“硬证据”）

### 2.1 可确证的系统能力（允许写入）

1. **服务端-客户端通信与协议封装**：`transporter` 定义了 `Transporter/Protocoler/Packager/Package` 分层，服务端通过 TCP 监听并对 SQL 文本执行后返回结果或错误。
1. **SQL 解析与执行**：`parser` 支持 `begin/commit/abort/create/drop/read/insert/delete/update/show`；`server/executor.go` 实现显式事务与“临时事务自动提交/回滚”的控制逻辑。
1. **存储与恢复（DM）**：`dm` 管理数据页缓存、日志记录与恢复；`recovery.go` 实现 `Redo/Undo` 的恢复流程；`pcacher` 管理页缓存与文件截断。
1. **事务管理（TM）**：`tm/transaction_manager.go` 通过 `.xid` 文件维护事务状态（active/committed/aborted），并持久化 `xidCounter`。
1. **并发控制与隔离（SM）**：`sm/serializability_manager.go` 注释声明“保障可串行化并实现 MVCC”，并包含死锁检测锁表（`sm/locktable`）与版本跳跃检查等逻辑。
1. **表管理与索引（TBM/IM）**：
   - `tbm` 提供建表、增删改查与 show；
   - `im/tree.go` 实现 B+Tree（基于 DM）并支持范围查询；源码注释提示参考 boltDB 思路，且存在并发更新根节点的 TODO/潜在一致性风险（论文中需要如实表述，不要夸大“完全无缺陷”）。

### 2.2 真实测试证据（允许写入“验证/测试”章节）

1. 后端核心模块单元/集成测试存在且可运行（见 `src/main/backend/**/**_test.go` 与 `test/`）。
1. 提供了客户端-后端集成测试与并发稳定性测试（`test/integration`、`test/concurrency`）。
1. 项目文档 `doc/测试文件说明.md` 已把关键测试目标与覆盖点写清楚，可转写为论文的测试方案与用例说明。

> 备注：我在当前环境通过设置 `GOCACHE=/tmp/...` 成功运行 `go test ./...`，所有测试包通过，可作为“本地验证证据”。（论文中建议截图 + 文字描述，不要捏造吞吐/性能指标。）

---

## 3. 论文大纲（建议 1-3 级标题）

> 说明：该大纲遵循学校“目录三级标题、结论不加章号”的常见要求；若你的模板对“结论是否编号”有明确样例，以模板为准。

### 摘要（中文）
### Abstract（英文）
### 目 录（自动生成）

### 第1章 引言
1.1 研究背景与意义（面向教学/课程项目的工程价值表述）  
1.2 国内外相关技术概述（数据库内核、事务、恢复、索引；只写通用知识，不写本系统不存在的功能）  
1.3 本文工作与组织结构

### 第2章 需求分析
2.1 系统目标与范围界定（轻量级关系型 DB 核心能力）  
2.2 业务用例与角色（CLI 用户/客户端程序、服务端）  
2.3 功能性需求（DDL/DML/事务控制/索引查询）  
2.4 非功能需求（正确性、可恢复性、并发一致性、可测试性；不写“高并发高可用”空泛口号）  
2.5 约束条件（本地文件存储、简化的 SQL 语法、教学性质）

### 第3章 总体设计
3.1 系统总体架构（Client/Server/Protocol/Engine）  
3.2 核心模块划分（TM/DM/SM/IM/TBM）  
3.3 关键数据与文件组织（`.xid`、日志文件、数据文件、booter 文件等）  
3.4 关键接口与调用链（`launcher -> server -> executor -> tbm -> sm -> dm/tm/im`）

### 第4章 关键实现
4.1 通信协议与封包机制（HexTransporter + Protocoler）  
4.2 SQL 解析与执行器设计（显式事务/临时事务）  
4.3 事务管理（XID 文件结构、状态流转）  
4.4 数据页与缓存管理（页结构、空闲空间选择、缓存引用计数）  
4.5 WAL/日志与恢复（Redo/Undo、崩溃恢复流程）  
4.6 并发控制与隔离（MVCC 可见性、锁表与死锁检测、版本跳跃）  
4.7 表管理与索引（表结构持久化、字段类型、B+Tree 范围查询）

### 第5章 测试与结果分析
5.1 测试环境与方法（单元/集成/并发稳定性）  
5.2 关键测试用例设计（从 `doc/测试文件说明.md` 与测试代码转写）  
5.3 测试结果与问题讨论（仅陈述“通过/失败与原因”，不写虚构性能图表）

### 结论
（总结实现内容、工程亮点、局限性与未来工作）

### 参考文献（GB/T 7714—2015）
### 附录（可选：SQL 语法、关键代码片段、测试脚本）
### 致谢

---

## 4. 章节证据映射（Claims -> Evidence）

> 使用方式：写每个小节前，先确认该小节每句话都能落到“证据文件”或“通用教材知识”。凡是涉及“本系统做了什么”，必须给出证据文件。

### 第2章 需求分析

- “系统支持的 SQL 语句范围”：证据：`src/main/backend/parser/parser.go`，`src/main/backend/parser/syntax.txt`，`src/main/backend/parser/statement/*`
- “支持事务 begin/commit/abort，并限制嵌套事务”：证据：`src/main/backend/server/executor.go`
- “支持建表/索引字段声明/增删改查”：证据：`src/main/backend/tbm/table_manager.go`，`src/main/backend/tbm/table.go`，`src/main/backend/tbm/field.go`

### 第3章 总体设计

- “客户端-服务端交互分层”：证据：`src/main/client/client/client.go`，`src/main/client/client/shell.go`，`src/main/transporter/*`，`src/main/backend/server/server.go`
- “系统启动方式与 DB create/open 流程”：证据：`src/main/backend/launcher/launcher.go`
- “核心模块五层结构（TM/DM/SM/IM/TBM）”：证据：`doc/Chpter1、模型概述.md` + 实际包路径 `src/main/backend/*`

### 第4章 关键实现

- 4.1 通信协议
  - “Hex 编码以规避特殊字符，并以换行作为帧边界”：证据：`src/main/transporter/transporter.go`
  - “用 flag 区分 data/err”：证据：`src/main/transporter/protocoler.go`
- 4.2 SQL 解析与执行
  - “token 驱动的解析入口与语句分派”：证据：`src/main/backend/parser/parser.go`
  - “显式事务状态机 + 临时事务封装”：证据：`src/main/backend/server/executor.go`
- 4.3 事务管理
  - “XID 文件头与每事务 1 字节状态”：证据：`src/main/backend/tm/transaction_manager.go`
- 4.4 数据管理与缓存
  - “Insert 选择页、空闲空间索引、写入日志再写页”：证据：`src/main/backend/dm/data_manager.go`
  - “通用缓存组件引用计数/并发共享”：证据：`src/main/backend/utils/cacher/*`
- 4.5 恢复
  - “Recovery 先扫描日志确定 maxPgno，truncate，再 redo/undo”：证据：`src/main/backend/dm/recovery.go`
- 4.6 并发控制与隔离
  - “SM 声明实现 MVCC 与可串行化保障；冲突时自动回滚”：证据：`src/main/backend/sm/serializability_manager.go`
  - “锁表维护等待图并死锁检测”：证据：`src/main/backend/sm/locktable/lock_table.go`
- 4.7 索引
  - “B+Tree bootUUID 指向 root，支持 range scan”：证据：`src/main/backend/im/tree.go`，`src/main/backend/im/node.go`

### 第5章 测试

- “模块单元测试覆盖缓存/页缓存/DM/索引/事务等”：证据：`src/main/backend/**/**_test.go`
- “端到端集成测试覆盖建表-增删改查-事务主流程”：证据：`test/integration/integration_test.go`
- “并发稳定性测试覆盖 50/100 并发客户端写读”：证据：`test/concurrency/concurrency_test.go`
- “测试设计说明文档”：证据：`doc/测试文件说明.md`

---

## 5. Keep / Rewrite / Delete 矩阵（当前输入资产）

> 说明：你目前没有提供“已写的论文初稿”。因此矩阵以“模板/规范/仓库材料”作为输入资产来做取舍；等你给出 working draft（docx）后，可再做一次“逐节 keep/rewrite/delete”。

| 资产/内容 | Keep（保留） | Rewrite（改写） | Delete（删除） | 证据/备注 |
|---|---:|---:|---:|---|
| 学校模板中的版式（标题层级、页眉页码样式、目录样式） | ✓ |  |  | 以模板为最终权威 |
| 模板里的提示文本框/说明（“阅后删除此文本框”） |  |  | ✓ | 定稿禁止出现过程性文字 |
| 学校“书写规范及装订要求”中的结构与格式条款 | ✓ |  |  | 作为排版与章节硬约束 |
| `doc/Chpter1、模型概述.md` |  | ✓ |  | 内容可用，但必须改成论文体、补齐细节并与代码一致 |
| `doc/测试文件说明.md` |  | ✓ |  | 可改写成“测试方案与用例表” |
| 代码注释中的主张（如“无死锁”“保证可串行化”） |  | ✓ |  | 需用实现细节与测试证据支撑，并避免绝对化措辞 |
| “高并发/高性能/海量数据/生产可用”等宣传性结论 |  |  | ✓ | 没有基准测试与部署证据，禁止写 |
| 参考案例 PDF 的章节表达方式 | ✓（仅作风格参考） |  |  | 不作为功能/数据来源 |

---

## 6. 图表与截图规划（工程论文风格）

### 6.1 必画（建议 6-10 张）

1. **系统总体架构图**（第3章）
   - Client（Shell/Client API）/Transporter/Server/Engine（TBM/SM/DM/TM/IM）
2. **模块依赖图**（第3章）
   - `TBM -> SM -> (TM, DM)`, `TBM -> IM`, `Server -> Executor -> TBM`, `Client -> RoundTripper -> Packager`
3. **通信与包格式图**（第4章）
   - HexTransporter 帧化 + Protocoler 的 `flag + payload`
4. **事务状态流转图**（第4章）
   - Begin/Commit/Abort 对 `.xid` 的状态位更新
5. **恢复流程图**（第4章）
   - Scan log -> truncate -> redo -> undo
6. **锁表等待图与死锁检测示意**（第4章）
   - “等待边加入 -> DFS 检测环 -> 允许/拒绝”
7. **B+Tree 结构与范围查询示意**（第4章）
   - bootUUID 指向 root；叶子链表范围扫描

### 6.2 建议的表格（3-6 张）

1. SQL 语句与语法要点表（第2章/第4章）
2. 系统文件后缀与用途表（第3章）：`.xid`、日志、DB、`.bt` 等（以 `launcher.go` 的存在性检查为证据）
3. 日志格式字段表（第4章）：InsertLog/UpdateLog 字段布局（从 `recovery.go` 推导）
4. 测试用例覆盖矩阵（第5章）：模块 vs 用例（从测试代码与 `doc/测试文件说明.md` 抽取）

### 6.3 运行截图（建议 6-12 张，全部来自真实运行）

1. 服务端启动日志（`launcher -open ...`）
2. 客户端 Shell 的交互截图（`create/insert/read/update/delete/show`）
3. 事务冲突/回滚的一次可复现实验截图（如可构造）
4. `go test ./...` 通过的终端截图（证明测试存在且可运行）

---

## 7. 禁止/高风险表述清单（定稿必须规避）

1. 任何吞吐/延迟/QPS/并发性能结论（除非你补做了可复现 benchmark，并提供测试脚本与原始数据）。
1. “生产可用”“已部署上线”“服务大量用户”等规模叙述。
1. “完全无死锁”“严格保证可串行化”等绝对化表述（可写“通过等待图检测降低死锁风险/在给定实现约束下保证…”并用代码与测试支撑）。
1. 不存在的功能：权限系统、SQL 优化器、复杂 Join、分布式、复制、二级索引多列组合等（除非代码确实实现）。

---

## 8. 给 academic-paper-composer 的可执行交接清单（下一步怎么写）

1. **先创建工作稿**：复制学校模板 docx 为你的“定稿工作文件”（不要在模板原件上改）。
1. **按本文件第3节大纲**补齐正文内容：每小节写作前先选定证据文件（第4节）。
1. **图表先画后写**：先把第6节“必画”图做成黑白工程风格，再在正文中引用（图注按章编号）。
1. **测试章节用“代码 + 截图 + 用例表”写实**：只写你确实跑过/能复现的内容。
1. **最后再做排版统一**：目录自动生成、三级标题、页眉页码、图表题注、参考文献格式。

### 建议补充输入（有了这些，composer 能更快产出完整论文）

1. 你的论文题目（中/英）、姓名/学院/专业/学号/指导老师/日期。
2. 是否要求“结论不编号”（以模板样例为准）。
3. 你的查重/AI 检测报告（如果有）：请提供热点页或原文片段，我可以按热点映射并给出重写优先级矩阵。

