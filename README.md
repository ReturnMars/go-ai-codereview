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

## 📦 安装 (Installation)

```bash
git clone https://github.com/yourusername/go-ai-reviewer.git
cd go-ai-reviewer
go mod tidy
go build -o reviewer cmd/reviewer/main.go
```

## 🛠️ 配置 (Configuration)

项目支持通过 CLI Flags、环境变量或配置文件进行配置。

推荐在项目根目录创建 `.code-review.yaml`：

```yaml
# LLM 配置
api_key: "sk-xxxxxxxxxxxxxxxx"      # 您的 API Key
model: "deepseek-chat"              # 模型名称
base_url: "https://api.deepseek.com" # API 地址 (DeepSeek, LocalAI 等)

# 运行配置
concurrency: 5                      # 并发 Worker 数量
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
go run cmd/reviewer/main.go run .
```

### 进阶参数

指定目录并过滤后缀：
```bash
go run cmd/reviewer/main.go run ./src --include .ts,.tsx --concurrency 10
```

### 查看帮助
```bash
go run cmd/reviewer/main.go help
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

