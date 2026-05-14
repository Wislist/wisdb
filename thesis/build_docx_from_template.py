#!/usr/bin/env python3
"""
Build a thesis working DOCX by rewriting the BIT Zhuhai College thesis template.

No external dependencies (no python-docx). We edit `word/document.xml` inside
the template DOCX:
- fill cover page fields
- fill Chinese/English abstracts + keywords
- replace the sample body with a multi-chapter body aligned to `mydb-go`
- replace conclusion, references, appendix, acknowledgements
- remove template guidance paragraphs containing "注：" or "阅后删除"

Note: the table of contents is an auto-field; update it in Word after opening.
"""

from __future__ import annotations

import copy
import os
import zipfile
import xml.etree.ElementTree as ET


W_NS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
NS = {"w": W_NS}


def p_text(p: ET.Element) -> str:
    parts: list[str] = []
    for t in p.findall(".//w:t", NS):
        if t.text:
            parts.append(t.text)
    return "".join(parts).strip()


def set_p_text(p: ET.Element, text: str) -> None:
    # Keep paragraph properties (style), replace runs with a single run.
    for r in list(p.findall("w:r", NS)):
        p.remove(r)
    r = ET.SubElement(p, f"{{{W_NS}}}r")
    t = ET.SubElement(r, f"{{{W_NS}}}t")
    if text[:1].isspace() or text[-1:].isspace():
        t.set("{http://www.w3.org/XML/1998/namespace}space", "preserve")
    t.text = text


def clone_ppr(src_p: ET.Element) -> ET.Element | None:
    ppr = src_p.find("w:pPr", NS)
    return copy.deepcopy(ppr) if ppr is not None else None


def new_p(text: str, ppr_tpl: ET.Element | None) -> ET.Element:
    p = ET.Element(f"{{{W_NS}}}p")
    if ppr_tpl is not None:
        p.append(copy.deepcopy(ppr_tpl))
    r = ET.SubElement(p, f"{{{W_NS}}}r")
    t = ET.SubElement(r, f"{{{W_NS}}}t")
    t.text = text
    return p


def is_guidance(text: str) -> bool:
    if not text:
        return False
    if "阅后删除" in text:
        return True
    if text.startswith("注："):
        return True
    return False


def is_empty_cover_placeholder(text: str) -> bool:
    placeholders = {
        "学    院：",
        "专    业：",
        "学生姓名：",
        "学    号：",
        "指导教师：",
    }
    return text in placeholders


def find_idx(paras: list[ET.Element], pred) -> int | None:
    for i, p in enumerate(paras):
        if pred(p_text(p)):
            return i
    return None


def find_body_child_index(body: ET.Element, pred) -> int | None:
    for i, el in enumerate(list(body)):
        if el.tag == f"{{{W_NS}}}p" and pred(p_text(el)):
            return i
    return None


def is_sample_toc_entry(text: str) -> bool:
    # The template contains a static sample TOC (e.g. "结 论3", "参考文献4", "摘 要I").
    # We want to keep the real TOC field and remove these sample lines.
    if not text:
        return False
    if text in {"摘　要I", "摘 要I", "AbstractII", "Abstract II"}:
        return True
    if text[-1:].isdigit():
        if any(key in text for key in ["结", "参考文献", "附", "致", "第", "1.", "2.", "3.", "4.", "5."]):
            return True
    return False


def build_docx(template_docx: str, out_docx: str) -> None:
    # User-provided cover fields
    zh_title = "基于go语言的kv型数据库"
    en_title = "A key-value database based on the Go programming language"
    college = "计算机学院"
    major = "软件工程"
    name = "张昊"
    sid = "220021101674"
    advisor = ""  # per user: leave empty
    date_zh = "2026 年 4 月 16 日"

    zh_abs = (
        "键值（Key-Value）数据库以简单的数据模型和较高的读写效率，被广泛用于缓存、会话管理与轻量级存储场景。"
        "围绕教学与工程实践需求，本文基于 Go 语言设计并实现了一套轻量级键值型数据库原型系统。"
        "系统采用客户端/服务端结构，通过自定义协议在网络上传输 SQL 风格指令，并在服务端完成解析与执行。"
        "在存储层，系统实现了页式文件管理与缓存机制，采用日志记录支持崩溃恢复，并通过事务管理器维护事务状态。"
        "在并发控制方面，系统结合多版本可见性规则与锁表机制提升并发访问下的正确性。"
        "在索引方面，系统实现了基于数据管理模块的 B+ 树结构，支持等值与范围检索。"
        "最后，本文通过单元测试、端到端集成测试与并发稳定性测试对核心流程进行验证。"
    )
    zh_kw = "关键词：" + "；".join(["键值数据库", "Go语言", "事务管理", "日志恢复", "B+树", "并发控制"])

    en_abs = (
        "Key-value databases are widely used in lightweight storage scenarios such as caching and session management "
        "because of their simple data model and efficient read/write operations. This thesis presents the design and "
        "implementation of a lightweight key-value database prototype in Go. The system adopts a client-server "
        "architecture and transfers SQL-like commands via a custom protocol. It implements paged file management and a "
        "cache layer, records logs for crash recovery, maintains transaction states, and provides concurrency control "
        "with multi-version visibility rules and a lock-table mechanism. A B+ tree index supports point and range "
        "queries. Unit tests, end-to-end integration tests, and concurrent stability tests are conducted to verify the "
        "core workflows."
    )
    en_kw = "Key Words: " + "; ".join(
        ["key-value database", "Go", "transaction management", "log recovery", "B+ tree", "concurrency control"]
    )

    body_paras: list[tuple[str, str]] = [
        ("chap", "第1章 引言"),
        ("sec", "1.1 研究背景与意义"),
        ("p", "随着互联网应用场景持续扩展，缓存、会话、配置中心和轻量级业务存储等场景都需要结构简单、访问频繁且易于嵌入式部署的数据管理组件。键值型数据库因数据模型直接、读写路径短而被广泛采用，也是理解数据库内核设计的重要切入点。相较于直接调用成熟数据库产品，从零开始实现一个包含事务、恢复、索引和并发控制能力的数据库原型，更有助于将课堂中的事务理论、存储管理和索引结构知识转化为可验证的工程实现。"),
        ("p", "本课题以 Go 语言为实现基础。Go 语言具备较好的并发支持、跨平台部署能力和较清晰的标准库网络接口，适合用于构建教学型数据库系统原型。围绕这一特点，本文结合 mydb-go 项目的真实代码，梳理系统从命令输入、协议封装、语法解析、事务调度到页式持久化的完整处理链路，并通过已有测试对关键行为进行验证。"),
        ("sec", "1.2 研究内容与本文工作"),
        ("p", "本文研究的对象是一个基于 Go 语言实现的轻量级数据库原型系统。该系统采用客户端/服务端架构，客户端通过 Shell 或可复用的 API 发送 SQL 风格命令，服务端完成解析、执行和结果返回。围绕数据库内核的核心问题，项目实现了事务管理器、数据管理器、可串行化管理器、表管理器以及索引管理器等模块。"),
        ("p", "在具体工作上，本文首先分析系统的功能边界和工程约束，其次梳理模块划分和文件组织方式，再对事务状态持久化、日志恢复、多版本可见性、等待图死锁检测以及 B+ 树范围查询等关键实现进行说明。最后，本文结合单元测试、端到端集成测试与并发稳定性测试，对项目在给定实现范围内的正确性进行讨论。"),
        ("sec", "1.3 论文组织结构"),
        ("p", "第1章介绍课题背景、研究内容与论文结构；第2章从功能与非功能两个方面分析系统需求；第3章说明系统总体架构、模块划分以及核心调用链；第4章详细阐述通信协议、SQL 解析执行、事务、存储恢复、并发控制和索引实现；第5章给出测试方法、关键用例与结果分析；最后对全文进行总结，并说明后续可改进的方向。"),
        ("chap", "第2章 需求分析"),
        ("sec", "2.1 系统目标与范围"),
        ("p", "mydb-go 的目标不是实现一个生产级数据库，而是在本地文件存储场景下构建一套可运行、可测试、可讲解的数据库原型系统。系统需要支持通过网络接收命令，并在服务端完成数据定义、数据操作和事务控制等处理；同时还需要具备最基本的崩溃恢复能力，以保证异常关闭后能够通过日志和事务状态恢复到一致状态。"),
        ("p", "从实现范围看，系统聚焦于教学型数据库内核所需的核心链路，不引入分布式复制、复杂 SQL 优化器、权限系统与图形化管理界面等扩展能力。该范围界定能够让系统复杂度维持在可控区间内，也使论文叙述更贴合现有代码证据。"),
        ("sec", "2.2 功能性需求"),
        ("p", "结合解析器与表管理器的实现，系统在功能上需要支持 begin、commit、abort 等事务控制语句，支持 create、drop、show 等表级操作，并支持 insert、read、update、delete 等基础数据操作。为了满足基于主键或索引字段的快速访问需求，系统还需要允许在建表阶段声明索引字段，并在查询条件满足时执行等值或范围检索。"),
        ("p", "除语法支持外，执行层还应处理会话级事务上下文。对显式 begin 开启的事务，系统需要限制嵌套事务并提供显式提交或回滚；对未显式开启事务的单条语句，执行器需要自动补全临时事务，在语句成功时提交，在出现异常时回滚，以降低客户端使用门槛。"),
        ("sec", "2.3 非功能性需求"),
        ("p", "正确性是系统最重要的非功能性需求。事务状态需要被持久化记录，崩溃后需要根据日志和事务状态对数据页执行重做或撤销；在并发访问场景下，系统应通过可见性判断和锁表机制尽量避免脏写和不可串行化的更新结果。"),
        ("p", "可测试性也是本项目的重要要求。系统需要将协议层、执行层、数据管理层和缓存层设计为可分别测试的组件，并通过 t.TempDir 等隔离机制为测试用例创建独立环境，避免文件污染。与此同时，系统还应保留较清晰的模块边界，以便后续补充新的语句、测试或恢复场景。"),
        ("sec", "2.4 约束条件与设计取舍"),
        ("p", "受项目定位影响，系统当前实现采用本地文件作为持久化介质，默认通过 TCP 端口提供服务，且 SQL 语法经过了较大幅度的简化。系统没有实现复杂连接查询、多列联合索引、成本优化器或副本同步机制，因此论文中仅讨论已经实现的工程能力，而不对不存在的能力做延伸性结论。"),
        ("p", "在设计取舍上，项目更强调模块职责清晰与关键流程闭环，而不是追求极端性能指标。例如，通信层使用十六进制编码换取协议实现的简洁性，恢复模块优先保证恢复流程可运行，再通过 TODO 和测试建议暴露后续可改进之处。这些取舍符合课程项目和本科毕业设计的实现目标。"),
        ("chap", "第3章 总体设计"),
        ("sec", "3.1 系统总体架构"),
        ("p", "系统整体采用客户端/服务端架构。客户端侧包括交互式 Shell、RoundTripper 和 Client API，用于组织请求并展示结果；服务端侧由 TCP Server 接收连接，再把收到的命令交给执行器处理。执行器根据语句类型调用表管理器、事务管理器、数据管理器和索引模块，最终将执行结果重新编码后返回客户端。"),
        ("p", "这一架构将用户输入、网络传输、协议编解码与数据库核心逻辑分离开来，既降低了模块之间的耦合，也让集成测试可以通过 net.Pipe 的方式在内存中模拟客户端与服务端交互。对毕业论文而言，这种分层结构有利于从架构、调用链和模块职责三个角度展开叙述。"),
        ("sec", "3.2 核心模块划分"),
        ("p", "事务管理模块 TM 负责为每个事务分配唯一标识，并在 .xid 文件中持久化事务状态；数据管理模块 DM 负责页式文件、数据项写入、日志记录和恢复；可串行化管理模块 SM 负责事务可见性判断、锁表协调和冲突回滚；索引管理模块 IM 基于 DM 提供 B+ 树结构；表管理模块 TBM 负责表结构、字段定义以及面向上层的增删改查接口。"),
        ("p", "从依赖关系看，TBM 处于靠近业务语义的一层，它在执行建表和数据操作时依赖 SM 与 IM；SM 再向下依赖 TM 和 DM 获取事务状态与底层数据；IM 基于 DM 提供键到记录地址的映射；TM 与 DM 分别管理事务元信息和数据页内容。这样的设计既体现了数据库内核的层次性，也便于后续在某一层单独扩展。"),
        ("sec", "3.3 文件组织与启动流程"),
        ("p", "系统通过 launcher 提供 create 和 open 两种启动模式。创建数据库时，程序依次创建事务状态文件、数据文件以及引导表链的 booter 文件；打开数据库时，则依次加载 TM、DM、SM 与 TBM，再启动网络服务。根据启动器中的存在性检查，数据库目录至少会维护数据文件、日志文件、.xid 文件和 .bt 引导文件。"),
        ("p", "这一文件组织方式与后续恢复流程直接相关。事务状态文件用于判断事务是否处于 active、committed 或 aborted 状态；日志文件保存数据页更新记录；数据文件保存页面内容；booter 文件用于定位表链或索引树入口。模块启动顺序与文件职责相互配合，形成从磁盘恢复到服务上线的完整初始化链路。"),
        ("sec", "3.4 核心调用链设计"),
        ("p", "当客户端发送命令时，请求首先经过 Transporter 和 Protocoler 完成网络传输与包编码，然后由服务端 Executor 调用 parser.Parse 识别语句类型。若语句属于 begin、commit 或 abort，执行器会直接更新会话事务状态；若属于普通 DDL/DML 语句，则执行器会在必要时自动创建临时事务，并转交 TBM 继续处理。"),
        ("p", "TBM 在处理具体语句时，会根据表结构把请求下发给 SM、IM 和 DM。例如，插入语句需要根据字段定义完成类型编码，再通过 SM 插入数据项，必要时同步更新索引；查询语句会根据 where 条件选择直接扫描或借助 B+ 树定位记录。整个调用链虽然保持简化，但已经覆盖从网络输入到持久化输出的关键路径。"),
        ("chap", "第4章 关键实现"),
        ("sec", "4.1 通信协议与封包机制"),
        ("p", "通信层由 Transporter、Protocoler 和 Packager 组成。Transporter 为避免原始二进制数据中出现换行等控制字符导致粘包或分帧困难，先将数据编码为十六进制文本，再在末尾补充换行符，使接收端能够通过 ReadBytes('\\n') 读取完整报文。虽然这种做法会增加报文长度，但协议逻辑简洁、易于调试，适合课程项目原型。"),
        ("p", "Protocoler 进一步约定了数据包格式，即使用 1 字节 flag 区分正常结果与错误信息。当 flag 为 0 时，后续载荷表示执行结果；当 flag 为 1 时，载荷表示错误文本。该机制使客户端与服务端能够复用同一套收发逻辑，避免为异常场景单独设计额外协议。"),
        ("sec", "4.2 SQL 解析与执行器设计"),
        ("p", "解析器以 tokener 为基础，通过 Peek 和 Pop 的方式逐个读取 token，并在 parser.Parse 中按首关键字分派到不同的语句解析函数。当前解析器支持 begin、commit、abort、create、drop、read、insert、delete、update 和 show 等语句，满足系统的基本数据定义与数据操作需求。"),
        ("p", "执行器维护一个与客户端会话绑定的 xid 字段，用于记录当前是否处于显式事务中。当收到 begin 命令时，执行器会检查是否已在事务内，从而阻止嵌套事务；当收到 commit 或 abort 时，则要求当前存在有效事务。对普通语句，执行器在无显式事务的情况下会开启临时事务，并在语句执行结束后依据 err 状态自动提交或回滚。该机制在简化客户端使用的同时，也避免了大量语句因遗漏 begin 而失去事务保护。"),
        ("sec", "4.3 事务管理器设计"),
        ("p", "事务管理器以 .xid 文件为核心数据结构。文件头部保存事务计数器，之后按顺序为每个事务分配 1 字节状态位，其中 0 表示 active，1 表示 committed，2 表示 aborted。Begin 操作先将新事务状态写为 active，再更新计数器；Commit 和 Abort 则分别更新对应事务的状态位，并通过文件同步保证元信息落盘。"),
        ("p", "通过这种设计，TM 在实现上保持了较低复杂度，同时为恢复模块提供了清晰的判断依据。恢复时只需读取事务状态，即可区分哪些日志需要重做，哪些日志属于崩溃时尚未结束的事务，需要按逆序撤销。论文中把 TM 单独作为一节展开，能够较好体现数据库系统中事务元信息持久化的基础作用。"),
        ("sec", "4.4 数据管理器与缓存机制"),
        ("p", "数据管理模块围绕页式存储展开。记录写入时，DM 需要先根据页面空闲空间选择合适页，再生成对应日志并把数据项写入页面，最终返回逻辑地址供上层模块引用。页面级缓存和通用对象缓存通过引用计数管理对象获取与释放，以减少重复加载并控制共享对象的生命周期。"),
        ("p", "这种设计体现了数据库底层存储的两个关键目标：其一是通过页作为读写基本单位，把磁盘访问组织为较稳定的数据结构；其二是通过缓存降低频繁磁盘 I/O 的代价。项目测试中对 DataManager、Pcacher 与通用缓存组件分别进行了单元测试，为论文提供了较直接的验证证据。"),
        ("sec", "4.5 日志记录与崩溃恢复"),
        ("p", "恢复模块实现了较完整的启动恢复链路。系统启动后会先扫描日志文件，找出日志中出现过的最大页号，并据此截断数据文件，避免由于异常关闭导致页数量与日志状态不一致。随后，恢复流程执行 Redo 与 Undo 两个阶段：对已完成事务的日志执行重做，对崩溃时仍处于 active 状态的事务日志按逆序撤销，并最终把这些事务标记为 aborted。"),
        ("p", "在日志内容上，项目区分了插入日志和更新日志。插入日志记录事务标识、页号、偏移和原始数据，更新日志记录事务标识、数据项地址、旧值和新值。通过这种日志格式，系统可以在恢复阶段复原页面状态。源码中也保留了关于页空闲空间指针异常的 TODO 注释，这说明当前恢复实现已经具备教学意义上的完整闭环，但仍存在可继续打磨的边界场景。"),
        ("sec", "4.6 并发控制与可见性"),
        ("p", "可串行化管理模块 SM 同时承担 MVCC 可见性判断与写冲突协调职责。对读取操作，系统会先把底层数据包装为 entry，并根据事务级别、版本边界和事务状态判断当前事务是否可见；对删除或更新类写操作，系统则需要在可见性判断基础上进一步申请锁表中的记录锁。"),
        ("p", "锁表模块为每个事务维护等待关系，并在加入等待边时检测是否形成环。一旦发现存在无法序列化的更新路径，系统会设置事务错误状态并执行回滚。与只依赖互斥锁的简单并发控制相比，这一设计更接近数据库课程中关于版本可见性和死锁检测的核心内容，也为论文中“并发控制”章节提供了较充分的实现依据。"),
        ("sec", "4.7 表管理与索引实现"),
        ("p", "表管理模块 TBM 面向上层暴露 create、drop、show、insert、read、update 和 delete 等接口。其内部通过 booter 保存第一张表的标识，并以链表方式组织所有表结构；当建表或删表时，TBM 会更新这条表链。字段定义、表元数据和数据操作逻辑分别由 table.go 与 field.go 等文件配合完成。"),
        ("p", "索引管理模块 IM 在 DM 之上实现 B+ 树。每棵树通过 bootUUID 指向固定的根入口，根节点变化时只需要更新 boot 所指向的地址，而不必让外部持有根节点地址。查询时，系统先沿非叶节点向下检索，再在叶子链表上完成范围扫描；插入时若发生节点分裂，则可能继续向上递归并更新根节点。源码中明确说明根节点并发更新仍存在待完善问题，因此本文对索引模块的评价以“已实现基本等值与范围检索能力”为主，不夸大其并发安全性。"),
        ("chap", "第5章 测试与结果分析"),
        ("sec", "5.1 测试环境与方法"),
        ("p", "项目使用 Go 自带测试框架组织自动化测试。底层模块测试主要围绕 DM、Pcacher、通用缓存、事务管理、索引和执行器等组件展开，测试时通过 t.TempDir 创建隔离目录，保证每组用例拥有独立的数据库文件。对于协议层和服务端执行链路，则使用 net.Pipe 在内存中构造客户端与服务端连接，以模拟真实交互而不依赖外部网络环境。"),
        ("p", "测试方法上，本文把验证过程分为三类：一是面向单个模块行为的单元测试，重点验证插入、读取、缓存释放、参数边界和异常情况；二是面向完整调用链的集成测试，验证建表、增删改查与事务控制是否协同工作；三是面向多客户端读写过程的并发稳定性测试，用于观察系统在 50 到 100 个并发客户端下的基本行为。"),
        ("sec", "5.2 关键测试用例分析"),
        ("p", "在数据管理模块测试中，项目覆盖了正常插入读取、超大数据插入报错、无效数据项读取以及并发插入读取等场景。页面缓存测试则覆盖了页面创建、脏页落盘、重启后重新加载和文件截断等行为；通用缓存测试关注相同键值并发读取是否复用同一底层对象，以及引用归零后是否能够重新创建对象。"),
        ("p", "端到端集成测试构造了一条较完整的业务路径：客户端依次执行 begin、create table、insert、read、update、delete、commit 和 show，并检查返回结果是否符合预期。并发稳定性测试则在 50 和 100 个并发客户端场景下执行插入与查询操作，并在所有子任务完成后再次逐条校验读取结果，从而验证协议层、执行器、表管理和底层存储链路在给定测试规模内能够保持基本一致性。"),
        ("sec", "5.3 测试结果与讨论"),
        ("p", "结合仓库中的测试代码和本地执行结果可以看出，项目已经具备较好的自动化验证基础。核心测试覆盖了数据页读写、缓存释放、SQL 执行路径和并发场景下的读写正确性，说明系统在本科毕业设计的实现范围内已经形成从模块到集成链路的闭环。"),
        ("p", "需要说明的是，当前测试更多聚焦功能正确性与基本稳定性，而非性能评估。项目尚未提供系统化的吞吐、延迟或故障注入基准，因此本文不对其生产场景表现做结论。后续工作可进一步补充高强度压力测试、恢复边界条件测试以及索引并发更新问题的回归测试，以提升系统的完整性。"),
    ]

    conclusion = [
        "本文以 mydb-go 项目为基础，围绕轻量级数据库原型的设计与实现展开研究，完成了通信协议、SQL 风格解析与执行、事务管理、页式存储、日志恢复、并发控制以及 B+ 树索引等关键模块的梳理与说明。",
        "从实现结果看，系统已经能够在客户端/服务端架构下完成建表、增删改查与事务控制等核心流程，并借助 .xid 文件、日志与恢复逻辑在给定范围内维持数据一致性。同时，项目通过单元测试、集成测试与并发稳定性测试对主要模块进行了验证，体现出较好的教学实验价值和工程可读性。",
        "受项目定位与时间成本限制，系统仍存在若干可改进之处，例如 SQL 语法能力较为有限，性能指标尚未做系统基准评估，B+ 树根节点并发更新与恢复边界条件也有继续完善空间。后续可从补充基准测试、增强异常注入、完善索引并发处理和扩展查询语法等方向进一步优化。"
    ]

    references = [
        "[1] Gray J, Reuter A. Transaction Processing: Concepts and Techniques[M]. San Francisco: Morgan Kaufmann, 1993.",
        "[2] Silberschatz A, Korth H F, Sudarshan S. Database System Concepts[M]. 7th ed. New York: McGraw-Hill, 2019.",
        "[3] Bernstein P A, Newcomer E. Principles of Transaction Processing[M]. 2nd ed. San Francisco: Morgan Kaufmann, 2009.",
        "[4] Bayer R, McCreight E. Organization and maintenance of large ordered indexes[J]. Acta Informatica, 1972, 1(3): 173-189.",
        "[5] Kleppmann M. Designing Data-Intensive Applications[M]. Sebastopol: O'Reilly Media, 2017.",
        "[6] The Go Authors. The Go Programming Language Specification[EB/OL]. https://go.dev/ref/spec, 2026-04-16.",
        "[7] The Go Authors. Package testing[EB/OL]. https://pkg.go.dev/testing, 2026-04-16.",
    ]

    appendix = [
        "附录A SQL 风格命令摘要",
        "（1）事务控制命令：begin、commit、abort。",
        "（2）表定义命令：create table <name> <field type,...> (index <field,...>)、drop table <name>、show。",
        "（3）数据操作命令：insert into <table> values ...、read * from <table> where ...、update <table> set ... where ...、delete from <table> where ...。",
        "（4）测试相关文件主要分布于 src/main/backend 下的各模块测试文件，以及 test/integration、test/concurrency 目录。",
    ]

    acknowledgements = [
        "值此论文完成之际，谨向在毕业设计过程中给予我帮助与支持的老师、同学和家人表示衷心的感谢。",
        "感谢学院提供的学习与实验环境，感谢同学们在讨论与测试阶段的帮助。",
    ]

    os.makedirs(os.path.dirname(out_docx), exist_ok=True)
    with zipfile.ZipFile(template_docx) as zin:
        with zipfile.ZipFile(out_docx, "w", compression=zipfile.ZIP_DEFLATED) as zout:
            for item in zin.infolist():
                data = zin.read(item.filename)
                if item.filename != "word/document.xml":
                    zout.writestr(item, data)
                    continue

                root = ET.fromstring(data)
                body = root.find("w:body", NS)
                if body is None:
                    raise RuntimeError("word/document.xml missing w:body")

                paras = [el for el in list(body) if el.tag == f"{{{W_NS}}}p"]

                # Find style templates from the sample body.
                chap_tpl = next((p for p in paras if "第1章 一级题目" in p_text(p)), None)
                sec_tpl = next((p for p in paras if p_text(p) == "1.1 二级题目"), None)
                subsec_tpl = next((p for p in paras if p_text(p) == "1.1.1 三级题目"), None)
                norm_tpl = next((p for p in paras if p_text(p) == "正文……"), None)
                if chap_tpl is None or sec_tpl is None or subsec_tpl is None or norm_tpl is None:
                    raise RuntimeError("failed to find body style templates in DOCX")

                ppr_chap = clone_ppr(chap_tpl)
                ppr_sec = clone_ppr(sec_tpl)
                ppr_subsec = clone_ppr(subsec_tpl)
                ppr_norm = clone_ppr(norm_tpl)

                def ppr(kind: str) -> ET.Element | None:
                    return {
                        "chap": ppr_chap,
                        "sec": ppr_sec,
                        "subsec": ppr_subsec,
                        "p": ppr_norm,
                    }.get(kind, ppr_norm)

                # Replace cover + abstracts anchors.
                for p in paras:
                    t = p_text(p)
                    if t == "毕业设计（论文）题目":
                        set_p_text(p, zh_title)
                    elif "The Subject of Undergraduate Graduation Project" in t:
                        set_p_text(p, en_title)
                    elif t == "学    院：":
                        set_p_text(p, f"学    院：{college}")
                    elif t == "专    业：":
                        set_p_text(p, f"专    业：{major}")
                    elif t == "学生姓名：":
                        set_p_text(p, f"学生姓名：{name}")
                    elif t == "学    号：":
                        set_p_text(p, f"学    号：{sid}")
                    elif t == "指导教师：":
                        set_p_text(p, f"指导教师：{advisor}")
                    elif t.startswith("20XX") and "年" in t and "月" in t and "日" in t:
                        set_p_text(p, date_zh)
                    elif t == "本文……。":
                        set_p_text(p, zh_abs)
                    elif t.startswith("关键词："):
                        set_p_text(p, zh_kw)
                    elif t == "In order to study……":
                        set_p_text(p, en_abs)
                    elif t.startswith("Key Words:"):
                        set_p_text(p, en_kw)

                # Ensure the cover info block exists (学院/专业/姓名/学号/指导教师).
                # Some template variants may group these placeholders with guidance text;
                # we insert a fresh block before the date line.
                cover_ppr = None
                for cand in paras:
                    if p_text(cand).startswith("学") and "院" in p_text(cand) and "：" in p_text(cand):
                        cover_ppr = clone_ppr(cand)
                        break
                if cover_ppr is None:
                    date_p = next((p for p in paras if date_zh in p_text(p)), None)
                    if date_p is not None:
                        cover_ppr = clone_ppr(date_p)

                i_date = find_idx(paras, lambda tx: tx == date_zh)
                cover_exists = any(p_text(p) == f"学    院：{college}" for p in paras)
                if i_date is not None and cover_ppr is not None and not cover_exists:
                    cover_lines = [
                        f"学    院：{college}",
                        f"专    业：{major}",
                        f"学生姓名：{name}",
                        f"学    号：{sid}",
                        f"指导教师：{advisor}",
                    ]
                    # Insert based on the real body-child index (body can contain tables).
                    date_p = paras[i_date]
                    insert_at = list(body).index(date_p)
                    for line in cover_lines:
                        body.insert(insert_at, new_p(line, cover_ppr))
                        insert_at += 1

                # Remove guidance paragraphs.
                for p in list(paras):
                    text = p_text(p)
                    if is_guidance(text) or is_empty_cover_placeholder(text):
                        body.remove(p)

                # Refresh paragraph list.
                paras = [el for el in list(body) if el.tag == f"{{{W_NS}}}p"]

                # Remove static sample TOC entries (keep the actual TOC field).
                for p in list(paras):
                    if is_sample_toc_entry(p_text(p)):
                        body.remove(p)

                paras = [el for el in list(body) if el.tag == f"{{{W_NS}}}p"]

                # Replace sample body and remove any example tables/paragraphs between body anchor and conclusion.
                i_body_child = find_body_child_index(body, lambda t: t.strip() == "1.1 二级题目")
                i_conc_child = find_body_child_index(body, lambda t: t.replace(" ", "") in {"结论", "结　论"} and not any(ch.isdigit() for ch in t))
                if i_body_child is None or i_conc_child is None or i_body_child >= i_conc_child:
                    raise RuntimeError("failed to locate body/conclusion anchors")
                insert_at = i_body_child
                for el in list(body)[i_body_child:i_conc_child]:
                    if el.tag != f"{{{W_NS}}}sectPr":
                        body.remove(el)
                for k, txt in body_paras:
                    body.insert(insert_at, new_p(txt, ppr(k)))
                    insert_at += 1

                # Replace conclusion block (keep heading).
                paras = [el for el in list(body) if el.tag == f"{{{W_NS}}}p"]
                i_conc = find_idx(paras, lambda t: t.replace(" ", "") in {"结论", "结　论"} and not any(ch.isdigit() for ch in t))
                i_refs = find_idx(paras, lambda t: t.strip() == "参考文献")
                if i_conc is None or i_refs is None or i_conc >= i_refs:
                    raise RuntimeError("failed to locate conclusion/references anchors")
                conc_p = paras[i_conc]
                insert_at = list(body).index(conc_p) + 1
                for p in paras[i_conc + 1 : i_refs]:
                    body.remove(p)
                for txt in conclusion:
                    body.insert(insert_at, new_p(txt, ppr("p")))
                    insert_at += 1

                # Rebuild back-matter starting from the References heading:
                # References -> Appendix -> Acknowledgements.
                paras = [el for el in list(body) if el.tag == f"{{{W_NS}}}p"]
                i_refs = find_idx(paras, lambda t: t.strip() == "参考文献")
                if i_refs is None:
                    raise RuntimeError("failed to locate references heading")

                # Remove all paragraphs after the references heading (keep sectPr).
                for p in paras[i_refs + 1 :]:
                    body.remove(p)

                # Use the references heading style for other back-matter headings.
                refs_heading_ppr = clone_ppr(paras[i_refs])

                insert_pos = list(body).index(paras[i_refs]) + 1
                for txt in references:
                    body.insert(insert_pos, new_p(txt, ppr("p")))
                    insert_pos += 1

                body.insert(insert_pos, new_p("附　录", refs_heading_ppr))
                insert_pos += 1
                for txt in appendix:
                    body.insert(insert_pos, new_p(txt, ppr("p")))
                    insert_pos += 1

                body.insert(insert_pos, new_p("致　谢", refs_heading_ppr))
                insert_pos += 1
                for txt in acknowledgements:
                    body.insert(insert_pos, new_p(txt, ppr("p")))
                    insert_pos += 1

                zout.writestr(item, ET.tostring(root, encoding="utf-8", xml_declaration=True))


def main() -> None:
    template = "/Users/wislist/Documents/北京理工大学珠海学院本科生毕业设计（论文）模板（2022年9月）.docx"
    out = "/Users/wislist/Desktop/worksplace/mydb-go/thesis/张昊_基于go语言的kv型数据库_定稿工作稿.docx"
    build_docx(template, out)
    print(out)


if __name__ == "__main__":
    main()
