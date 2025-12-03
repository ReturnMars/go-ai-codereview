这是一个非常专业且具备实战指导意义的 `AGENTS.md` 文档。

不仅包含了功能定义，还融入了**架构设计、并发模型、工程化陷阱规避**以及**Prompt 工程**。这就好比我作为架构师，交给你（Team Leader）的一份详细的技术实施方案。

请将以下内容保存为 `AGENTS.md`，作为你项目的“宪法”。

---

# AGENTS.md - Project Blueprint: Go-AI-Reviewer

> **Role:** Senior Go Architect & Engineering Strategist
> **Objective:** Build a high-performance, CLI-based static code analysis tool powered by LLM.
> **Philosophy:** Engineering over Hype. Performance over Features.
> **Status:** Draft / Phase 0

## 1. 项目愿景 (Vision)

构建一个**开发者友好**、**零依赖分发**的命令行工具。它不是简单的 API 转发器，而是一个具备**并发控制**、**智能过滤**、**增量检查**能力的工程化产品。它旨在解决 Code Review 中的人力瓶颈，通过 AI 提供初步的代码质量评分与优化建议。

## 2. 技术栈选型 (Tech Stack Strategy)

我们不追求最时髦，只追求最**稳定**和**高效**。

| 组件              | 选型                                 | 理由 (The Why)                                                     |
| :---------------- | :----------------------------------- | :----------------------------------------------------------------- |
| **CLI Framework** | `github.com/spf13/cobra`             | Go 生态事实标准，结构清晰，支持子命令。                            |
| **TUI / UX**      | `github.com/charmbracelet/bubbletea` | 颜值即正义。提供现代化、交互式的终端体验（进度条、Spinner）。      |
| **Styling**       | `github.com/charmbracelet/lipgloss`  | 配合 Bubbletea，做漂亮的终端着色和布局。                           |
| **File Walking**  | `path/filepath` (Std Lib)            | 标准库足够好，配合自定义过滤逻辑。                                 |
| **Gitignore**     | `github.com/sabhiram/go-gitignore`   | **关键组件**。必须尊重`.gitignore`，否则扫描`node_modules`是灾难。 |
| **Concurrency**   | Native `Goroutines` + `Channels`     | 核心练习点。手写 Worker Pool，不依赖第三方重型调度库。             |
| **AI Client**     | `github.com/sashabaranov/go-openai`  | 社区最成熟的 OpenAI 接口封装，支持 Stream 和 Context。             |
| **Env Config**    | `github.com/spf13/viper`             | **架构升级**。Cobra 的最佳拍档，统一管理 CLI Flags、环境变量和配置文件。 |

---

## 3. 核心架构设计 (Architecture)

采用 **生产者-消费者 (Producer-Consumer)** 模型，避免单线程阻塞和 API 速率限制。

```mermaid
graph TD
    A[CLI Entry] -->|Init| B(Config & Flags)
    B --> C[File Scanner (Producer)]
    C -->|Filter .git/node_modules| D{File Valid?}
    D -- Yes --> E[Jobs Channel]
    D -- No --> F[Skip]

    E --> G[Worker Pool (5-10 Workers)]
    G -->|Request| H[AI Service API]
    H -->|Response| G
    G -->|Result| I[Results Channel]

    I --> J[Aggregator / TUI Renderer]
    J --> K[Final Report]
```

### 3.1. 推荐目录结构 (Project Layout)

遵循 Standard Go Project Layout，保持代码整洁。

```text
go-ai-reviewer/
├── cmd/
│   └── reviewer/
│       └── main.go        # 入口文件，仅负责调用 root command
├── internal/              # 私有代码，外部不可引用
│   ├── app/               # 核心业务逻辑 (Scanner, Reviewer)
│   ├── config/            # 配置加载
│   ├── ui/                # Bubbletea TUI 界面逻辑
│   └── llm/               # AI 客户端封装
├── pkg/                   # 可复用库 (如果有)
│   └── utils/             # 通用工具函数
├── .env.example           # 环境变量模版
├── config.yaml            # 主配置文件
├── skills.md              # 工具库技能点记录
├── project.md             # 项目实施日志
├── go.mod
└── README.md
```

---

## 4. 模块实施细节 (Implementation Details)

### 4.1. Phase 1: The Scanner (智能扫描器)

**目标：** 极速遍历，精准过滤。

- **输入：** 目标路径 (默认为 `.`)
- **核心逻辑：**
  1.  加载 `.gitignore` 规则。
  2.  **硬编码黑名单：** 即使没有 gitignore，也必须强制跳过 `.git`, `node_modules`, `dist`, `vendor`, `.idea`, `.vscode`。
  3.  **文件特征过滤：**
      - 跳过二进制文件 (检测前 512 字节是否有 0x00)。
      - 跳过超大文件 (如 > 50KB)，避免 Token 爆炸。
      - 仅包含特定后缀 (如 `.js`, `.ts`, `.go`, `.py`, `.java`)，通过 Flag 配置。

### 4.2. Phase 2: The Worker Pool (并发引擎)

**目标：** 榨干性能，但守住 API 限流底线。

- **结构定义：**

  ```go
  type Job struct {
      FilePath string
      Content  string
  }

  type Result struct {
      FilePath string
      Score    int
      Issues   []string
      Refined  string // 优化后的代码片段
      Error    error
  }
  ```

- **并发控制：**
  - 使用 `sync.WaitGroup` 等待所有 Worker 完成。
  - Worker 数量默认为 5 (可通过 `--concurrency` flag 调整)。
  - **Rate Limiting (进阶):** 在 Worker 内部实现简单的 Token Bucket 算法，或者简单的 `time.Sleep` 避免 429 错误。

### 4.3. Phase 3: The Brain (Prompt Engineering)

**目标：** 强迫 AI 输出机器可读的数据。

- **Prompt 策略：** 不聊天，只执行。
- **System Prompt:**
  ```text
  You are a senior code audit expert. Analyze the given code for logic errors, security vulnerabilities, and code style.
  Your output MUST be a strict JSON object without any Markdown formatting.
  Format:
  {
    "score": <0-100 integer>,
    "summary": "<1 sentence summary>",
    "issues": ["<critical issue 1>", "<issue 2>"],
    "suggestion": "<brief optimization advice>"
  }
  ```
- **容错与重试 (Retry Strategy):** 
  - 如果 JSON 解析失败，记录 Error 并自动重试一次（降低 Temperature）。
  - **Context Limit 保护:** 发送前估算 Token 数量，如果超出限制（如 4k/8k），自动对代码进行截断或仅发送 diff 部分。

### 4.4. Phase 4: The Interface (TUI)

**目标：** 让等待过程不枯燥。

- **Bubbletea Model:**
  - `Checking... [||||||    ] 60% (File: utils.go)`
  - 实时显示当前处理的文件名。
  - 完成后渲染表格或列表展示低分文件。

---

## 5. 开发路线图 (Roadmap)

### MVP (Minimum Viable Product) - _Day 1-3_

- [ ] 初始化 `go.mod` 及目录结构 (`cmd/`, `internal/`)。
- [ ] 集成 `cobra` 实现 CLI 骨架。
- [ ] 实现单文件读取与简单的 OpenAI API 调用。
- [ ] **关键点：** 跑通 "File -> Prompt -> Response -> Print" 流程。

### Beta (Concurrency & Filtering) - _Day 4-7_

- [ ] 实现 `filepath.Walk` + `.gitignore` 过滤。
- [ ] 实现 Worker Pool 并发模型。
- [ ] 接入 `Bubbletea` 显示简单的进度条。

### Production (Engineering Hardening) - _Day 8+_

- [ ] **Git Diff 模式：** 增加 `--diff` 参数，只检查本次 Git 修改的文件（极其实用）。
- [ ] **配置文件：** 支持 `.code-review.yaml` 配置忽略规则和 Prompt 模版。
- [ ] **Report 导出：** 支持生成 Markdown 或 HTML 报告。
- [ ] **Binary Build:** 使用 GitHub Actions 自动编译多平台二进制文件。

---

## 6. 避坑指南 (The Minefield)

1.  **Context Window Overflow:** 永远不要直接发送整个文件内容，除非你检查了它的大小。对于超长文件，要么截断，要么跳过，要么分片。
2.  **API Key 安全:** 哪怕是自己用的工具，也不要硬编码 Key。使用 `viper` 读取环境变量 `OPENAI_API_KEY`（支持自动加载 `.env`）。
3.  **JSON Hallucination:** AI 有时会发神经不返回 JSON。在 Go 里解析 JSON 一定要做好 `recover` 或者错误检查。
4.  **Channel Deadlocks:** 确保所有 Channel 都有 Sender 关闭，否则 Range 循环会死锁。Worker Pool 中，必须由主协程在 `WaitGroup.Wait()` 完成后关闭 `Results Channel`。
5.  **Graceful Shutdown:** 监听 `SIGINT` (Ctrl+C)，优雅地停止 Worker，保存当前已有的进度或报告，而不是直接崩溃。

---

## 7. 工作流与实施规范 (Workflow & Standards)

### 7.1. 知识库管理 (Knowledge Management)
- **Context7 Strategy:** 引入新第三方库前，必须通过 `Context7` 工具学习最新文档。
- **skills.md:** 实时维护项目用到的工具库列表、关键特性描述及使用场景。

### 7.2. 编码与设计 (Coding & Design)
- **Language:** 整个项目（代码注释、CLI 输出、Git Commit、文档）必须使用 **中文 (Chinese-Simplified)**。
- **Style:** 严格遵循 Go 官方标准。
- **Architecture:** 坚持 **单一职责原则 (Single Responsibility Principle)**。模块划分清晰，禁止代码耦合。
- **Configuration:** 统一使用 **YAML** 格式 (`config.yaml` 或 `.code-review.yaml`) 进行配置管理。

### 7.3. 测试策略 (Testing Strategy)
- **Scope:** 默认仅对 **工具函数 (`pkg/utils`)** 编写单元测试。
- **Exclusion:** 除非明确说明，业务逻辑层不强制要求单元测试，优先保证迭代速度。

### 7.4. 项目实施追踪 (Project Tracking)
- **project.md:** 项目实施日志。
- **Rule:** 每次功能模块实施完毕后，必须更新此文件，记录实施内容、变更详情及下一步计划。
