// Package llm 提供 LLM API 客户端封装
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// 常量定义
const (
	DefaultModel       = "deepseek-chat"
	DefaultTemperature = 0.2
	MinLevel           = 1
	MaxLevel           = 6
	DefaultLevel       = 3
)

// 系统提示模板
const systemPromptTemplate = `你是一位高级代码审计专家。请分析给定的代码，寻找逻辑错误、安全漏洞和代码风格问题。
你的输出必须是一个严格的 JSON 对象，不要包含任何 Markdown 格式（不要使用代码块）。
请使用中文回答。

**审查严格级别: %d/6**
%s

## 重要提示（避免误报）

1. **跨文件依赖**：你只能看到当前单个文件。如果代码调用了未在当前文件定义的函数/类/模块，它很可能定义在项目的其他文件中。**不要将"函数未定义"、"模块未导入"等报告为错误**，除非语法明显错误。

2. **语言特性**：
   - Go: 同 package 内文件可互相访问；init() 中 panic 是标准做法
   - Java: 同 package 类可互相访问；Spring/Maven 依赖注入；接口实现可能在其他模块
   - JavaScript/TypeScript: 模块可能通过 index.ts 重导出；框架有特定约定
   - Python: 相对导入、__init__.py 导出
   - Vue/React: 组件可能在其他文件注册

3. **框架设计模式**：每个框架有其设计约定，不要将框架的标准用法报告为问题。例如：
   - Elm 架构的 Update 用值类型是正确的
   - React Hooks 的依赖数组
   - Vue Composition API 的 ref/reactive

4. **只报告确定的问题**：如果某个问题依赖于你看不到的上下文（其他文件、配置、运行时），请不要报告。只报告在当前文件内**可以 100% 确定存在**的问题。

5. **区分严重程度**：
   - 语法错误、运行时崩溃、安全漏洞 = 严重问题（必须报告）
   - 代码风格、命名规范 = 一般建议（可以报告）
   - 基于假设的"可能问题" = **不要报告**

## 评估要求

评估该文件在项目中的重要性（0.0 - 1.0）：核心业务逻辑/入口=0.9~1.0，辅助工具=0.5，配置文件/简单模型=0.3。

格式：
{
  "score": <0-100 的整数>,
  "importance": <0.0-1.0 的浮点数，表示文件重要性>,
  "summary": "<一句话总结>",
  "pros": ["<优点 1>", "<优点 2>"],
  "issues": ["<确定存在的问题 1>", "<确定存在的问题 2>"],
  "suggestion": "<简短的优化建议>"
}`

// 级别描述映射
var levelDescriptions = map[int]string{
	1: `宽松模式：只关注严重的逻辑错误和安全漏洞。对代码风格和最佳实践不做要求。打分时给予较高分数，只有严重问题才扣分。`,
	2: `基础模式：关注明显的错误和潜在风险。对代码结构有基本要求。打分时对明显问题适度扣分。`,
	3: `标准模式：按常规代码审查标准进行评估。关注错误、风险、可读性和基本的最佳实践。打分时按标准尺度评分。`,
	4: `严格模式：对代码质量有较高要求。除了错误和风险，还要关注性能、可维护性和代码规范。打分时标准较严，小问题也需指出。`,
	5: `专业模式：按生产级代码标准审查。关注所有潜在问题，包括边界情况、异常处理、日志规范等。打分时非常严格，追求高质量代码。`,
	6: `极致模式：按顶级开源项目标准审查。任何不完美的地方都要指出，包括命名、注释、架构设计等。打分极其严格，90分以上必须是接近完美的代码。`,
}

// ReviewResult 表示 LLM 返回的结构化审查结果
type ReviewResult struct {
	Score      int      `json:"score"`      // 评分 (0-100)
	Importance float64  `json:"importance"` // 重要性 (0.0-1.0)
	Summary    string   `json:"summary"`    // 一句话总结
	Pros       []string `json:"pros"`       // 优点列表
	Issues     []string `json:"issues"`     // 问题列表
	Suggestion string   `json:"suggestion"` // 优化建议
}

// Client 封装 OpenAI API 客户端
type Client struct {
	api   *openai.Client
	model string
}

// NewClient 创建一个新的 LLM 客户端
func NewClient(apiKey, model, baseURL string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API Key 不能为空")
	}

	// 设置默认模型
	if model == "" {
		model = DefaultModel
	}

	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}

	return &Client{
		api:   openai.NewClientWithConfig(config),
		model: model,
	}, nil
}

// ReviewCode 发送代码给 LLM 并返回分析结果
func (c *Client) ReviewCode(ctx context.Context, filePath, content string, level int) (*ReviewResult, error) {
	// 验证并规范化 level
	level = normalizeLevel(level)

	// 构建提示词
	levelDesc := getLevelDescription(level)
	systemPrompt := fmt.Sprintf(systemPromptTemplate, level, levelDesc)
	userPrompt := fmt.Sprintf("File: %s\n\nCode:\n%s", filePath, content)

	// 调用 API
	resp, err := c.api.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: DefaultTemperature,
	})

	if err != nil {
		return nil, fmt.Errorf("API 调用失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("API 返回空响应")
	}

	// 解析响应
	return parseResponse(resp.Choices[0].Message.Content)
}

// parseResponse 解析 LLM 响应为 ReviewResult
func parseResponse(content string) (*ReviewResult, error) {
	// 使用正则表达式清理 Markdown 代码块
	// 匹配 ```json ... ``` 或 ``` ... ```
	// 使用非贪婪匹配 (.*?) 避免匹配到最后一个 ```
	codeBlockRegex := regexp.MustCompile("(?s)^\\s*```(?:json)?\\s*(.*?)```\\s*$")
	if matches := codeBlockRegex.FindStringSubmatch(content); len(matches) > 1 {
		content = matches[1]
	}

	content = strings.TrimSpace(content)

	// 如果内容为空，返回错误
	if content == "" {
		return nil, fmt.Errorf("响应内容为空")
	}

	var result ReviewResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// 不在错误信息中包含原始响应，避免泄露敏感信息
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	return &result, nil
}

// normalizeLevel 将 level 规范化到有效范围内
func normalizeLevel(level int) int {
	if level < MinLevel {
		return DefaultLevel
	}
	if level > MaxLevel {
		return DefaultLevel
	}
	return level
}

// getLevelDescription 获取级别对应的描述
func getLevelDescription(level int) string {
	if desc, ok := levelDescriptions[level]; ok {
		return desc
	}
	return levelDescriptions[DefaultLevel]
}

// EstimateTokenCount 估算文本的 Token 数量
// 注意：这是粗略估算（约 4 字符 = 1 Token），仅用于成本预估
// 精确计算请使用 tiktoken-go 等专业库
func EstimateTokenCount(text string) int {
	return len(text) / 4
}
