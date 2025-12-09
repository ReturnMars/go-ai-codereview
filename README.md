# Go AI Code Reviewer (AI 代码审查助手)

> **Engineering over Hype.** 一个高性能、工程化的 CLI 代码审查工具，由 LLM (DeepSeek/OpenAI) 驱动。

![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)
![License](https://img.shields.io/badge/license-MIT-blue.svg)

## ✨ 核心特性 (Features)

- **🚀 极速扫描**: 基于 Worker Pool 的并发架构，飞速处理海量文件。
- **🛡️ 智能过滤**: 自动识别 `.gitignore`，强制屏蔽 `node_modules` 等黑洞目录，并内置二进制文件检测。
- **🧠 AI 驱动**: 集成 DeepSeek/OpenAI，提供深度代码逻辑分析、安全漏洞检测和优化建议。
- **📊 专业报告**: 自动生成 Markdown 审查报告，支持 IDE 内点击跳转，包含综合评分与亮点分析。
- **🖥️ 交互体验**: 漂亮的 TUI 界面，实时展示扫描进度与状态 (Bubbletea powered)。
- **⚖️ 严格级别**: 6 级审查标准 (1=宽松, 6=极致)，根据项目需求灵活调整。

## 📦 安装 (Installation)

```bash
git clone https://github.com/yourusername/go-ai-reviewer.git
cd go-ai-reviewer
go mod tidy
go build -o reviewer cmd/reviewer/main.go
```

## 🛠️ 配置 (Configuration)

### 🚀 首次使用（自动配置）

首次运行时，工具会**自动引导**你输入配置信息并创建配置文件：

```bash
$ reviewer run .

🔧 首次使用，需要配置 API 信息
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📡 API Base URL [https://api.deepseek.com/v1]: 
🔑 API Key (必填): sk-xxxxxxxxxxxxxxxx
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ 配置已保存到 ~/.code-review.yaml
```

配置文件将自动创建在 `~/.code-review.yaml`，无需手动编辑。

### 手动配置（可选）

你也可以手动创建配置文件：

**用户主目录配置**（推荐，全局生效）：`~/.code-review.yaml`

**项目级配置**（可覆盖全局配置）：`./.code-review.yaml`

```yaml
# LLM 配置
api_key: "sk-xxxxxxxxxxxxxxxx" # 您的 API Key
model: "deepseek-chat" # 模型名称
base_url: "https://api.deepseek.com/v1" # API 地址 (DeepSeek, LocalAI 等)

# 运行配置
concurrency: 5 # 并发 Worker 数量
level: 2 # 默认审查级别 (1-6)
include_exts: [".go", ".js", ".ts", ".py"] # 仅扫描特定后缀 (留空扫描所有文本文件)
```

或者通过环境变量：

```bash
export OPENAI_API_KEY="sk-xxx"
```

## 🚀 使用指南 (Usage)

### 基础用法

扫描当前目录：

```bash
reviewer run .
```

### 进阶参数

指定目录并过滤后缀：

```bash
reviewer run ./src --include .ts,.tsx --concurrency 10
```

自定义报告名称：

```bash
# 生成 reports/my-audit.md
reviewer run . --rn my-audit
```

设置审查严格级别：

```bash
# 宽松模式 (只报严重问题)
reviewer run . --l 1

# 极致模式 (顶级代码标准)
reviewer run . --l 6
```

批量审查多个项目 (用位置参数为每个路径单独设置级别)：

```bash
# 格式: path [level] [report-name] path [level] [report-name] ...
# 第一个 level=5，第二个使用默认 level=2
reviewer run ./frontend 5 ./backend

# 两个都单独指定级别和报告名
reviewer run ./frontend 2 front-report ./backend 5 back-report

# 用 --l 设置全局默认级别（未单独指定的路径会使用此值）
reviewer run ./src 5 ./lib --l 4   # src=5, lib=4
```

### 命令参数详解

| 参数            | 别名   | 描述                                 | 默认值                      |
| :-------------- | :----- | :----------------------------------- | :-------------------------- |
| `--include`     | 无     | 仅扫描指定后缀的文件 (逗号分隔)      | (所有文本文件)              |
| `--concurrency` | 无     | 并发 Worker 数量                     | 5                           |
| `--report-name` | `--rn` | 自定义生成报告的文件名               | (目录名)                    |
| `--base-url`    | 无     | LLM API 地址 (用于 DeepSeek/LocalAI) | https://api.deepseek.com/v1 |
| `--l`           | 无     | 审查严格级别 (1-6)                   | 2                           |

### 严格级别说明

| 级别 | 名称     | 说明                                       |
| :--- | :------- | :----------------------------------------- |
| 1    | 宽松模式 | 只关注严重的逻辑错误和安全漏洞，打分宽松   |
| 2    | 基础模式 | 关注明显错误和潜在风险 **(默认)**          |
| 3    | 标准模式 | 常规代码审查标准                           |
| 4    | 严格模式 | 对代码质量有较高要求                       |
| 5    | 专业模式 | 按生产级代码标准审查                       |
| 6    | 极致模式 | 按顶级开源项目标准，追求完美               |

### 查看帮助

```bash
reviewer help
```

## 📂 报告样例 (Report Example)

审查完成后，会在 `reports/` 目录下生成 Markdown 报告：

```markdown
# 代码审查报告 (AI Powered)

## 📊 项目概览

- **项目综合评分:** 85.5 / 100
- **耗时:** 3.2s

## 🟢 [internal/app/scanner.go](../internal/app/scanner.go) (得分: 90 | 重要性: 0.8)

**总结:** 扫描器逻辑清晰，能正确处理黑名单过滤。

### ✅ 亮点

- 使用了 filepath.WalkDir 提高遍历性能。
- 内置了二进制文件头部检查。

### 💡 优化建议

建议将硬编码的 exclude 列表提取到配置文件中。
```

## 🏗️ 架构设计 (Architecture)

采用 **Producer-Consumer** 模型：

1.  **Scanner**: 遍历文件系统 -> Job Channel
2.  **Engine**: Worker Pool (并发 LLM 请求) -> Result Channel
3.  **Reporter**: 聚合结果 -> TUI 展示 & Markdown 生成

详见 [AGENTS.md](./AGENTS.md)。

## 🤝 贡献 (Contributing)

欢迎提交 Issue 和 PR！请确保代码风格符合 Go 标准。

## 📄 许可证 (License)

MIT License
