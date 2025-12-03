package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// ReviewResult represents the structured output from the LLM
type ReviewResult struct {
	Score      int      `json:"score"`
	Importance float64  `json:"importance"` // 0.0 - 1.0
	Summary    string   `json:"summary"`
	Pros       []string `json:"pros"`
	Issues     []string `json:"issues"`
	Suggestion string   `json:"suggestion"`
}

// Client wraps the OpenAI API client
type Client struct {
	api   *openai.Client
	model string
}

// NewClient creates a new LLM client
func NewClient(apiKey, model, baseURL string) *Client {
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	return &Client{
		api:   openai.NewClientWithConfig(config),
		model: model,
	}
}

// ReviewCode sends the code to LLM and returns the analysis result
func (c *Client) ReviewCode(ctx context.Context, filePath, content string, level int) (*ReviewResult, error) {
	// 根据级别生成严格程度描述
	levelDesc := getLevelDescription(level)

	// 1. Construct the prompt
	systemPrompt := fmt.Sprintf(`你是一位高级代码审计专家。请分析给定的代码，寻找逻辑错误、安全漏洞和代码风格问题。
你的输出必须是一个严格的 JSON 对象，不要包含任何 Markdown 格式（不要使用代码块）。
请使用中文回答。

**审查严格级别: %d/6**
%s

你需要评估该文件在项目中的重要性（0.0 - 1.0），例如：核心业务逻辑/入口=0.9~1.0，辅助工具=0.5，配置文件/简单模型=0.3。

格式：
{
  "score": <0-100 的整数>,
  "importance": <0.0-1.0 的浮点数，表示文件重要性>,
  "summary": "<一句话总结>",
  "pros": ["<优点 1>", "<优点 2>"],
  "issues": ["<严重问题 1>", "<问题 2>"],
  "suggestion": "<简短的优化建议>"
}`, level, levelDesc)

	userPrompt := fmt.Sprintf("File: %s\n\nCode:\n%s", filePath, content)

	// 2. Call OpenAI API
	resp, err := c.api.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: c.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userPrompt,
				},
			},
			Temperature: 0.2, // Low temperature for deterministic output
			// JSON Mode is supported in newer models (gpt-4-1106-preview etc.),
			// but we'll stick to text parsing for broader compatibility for now.
			// ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject},
		},
	)

	if err != nil {
		return nil, err
	}

	// 3. Parse Response
	contentResponse := resp.Choices[0].Message.Content

	// Clean up Markdown code blocks if present (common hallucination)
	contentResponse = strings.TrimPrefix(contentResponse, "```json")
	contentResponse = strings.TrimPrefix(contentResponse, "```")
	contentResponse = strings.TrimSuffix(contentResponse, "```")
	contentResponse = strings.TrimSpace(contentResponse)

	var result ReviewResult
	if err := json.Unmarshal([]byte(contentResponse), &result); err != nil {
		// Retry logic could go here (Task for Phase 4)
		return nil, fmt.Errorf("failed to parse JSON response: %v\nRaw: %s", err, contentResponse)
	}

	return &result, nil
}

// EstimateTokens simple token estimation (1 token ~= 4 chars)
// In production, use a proper tokenizer library like tiktoken-go
func EstimateTokens(text string) int {
	return len(text) / 4
}

// getLevelDescription 根据严格级别返回对应的审查标准描述
func getLevelDescription(level int) string {
	descriptions := map[int]string{
		1: `宽松模式：只关注严重的逻辑错误和安全漏洞。对代码风格和最佳实践不做要求。打分时给予较高分数，只有严重问题才扣分。`,
		2: `基础模式：关注明显的错误和潜在风险。对代码结构有基本要求。打分时对明显问题适度扣分。`,
		3: `标准模式：按常规代码审查标准进行评估。关注错误、风险、可读性和基本的最佳实践。打分时按标准尺度评分。`,
		4: `严格模式：对代码质量有较高要求。除了错误和风险，还要关注性能、可维护性和代码规范。打分时标准较严，小问题也需指出。`,
		5: `专业模式：按生产级代码标准审查。关注所有潜在问题，包括边界情况、异常处理、日志规范等。打分时非常严格，追求高质量代码。`,
		6: `极致模式：按顶级开源项目标准审查。任何不完美的地方都要指出，包括命名、注释、架构设计等。打分极其严格，90分以上必须是接近完美的代码。`,
	}

	if desc, ok := descriptions[level]; ok {
		return desc
	}
	return descriptions[3] // 默认标准模式
}
