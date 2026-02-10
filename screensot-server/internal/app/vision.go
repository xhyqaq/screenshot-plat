package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ImageEntry 代表单张图片及多个模型的识别结果
type ImageEntry struct {
	Base64       string
	ModelAnswers []ModelAnswer
}

type ModelAnswer struct {
	Model    string
	Question string
	Answer   string
	Raw      string
	Error    string
}

// analyzeImages 对每张图片并发调用多个模型，返回聚合结果。
func (a *App) analyzeImages(ctx context.Context, base64Images []string) []ImageEntry {
	// 模型列表：可通过环境变量覆盖，逗号分隔
	models := a.cfg.Models

	items := make([]ImageEntry, len(base64Images))
	var wg sync.WaitGroup
	wg.Add(len(base64Images))

	for i := range base64Images {
		i := i
		go func() {
			defer wg.Done()
			entry := ImageEntry{Base64: base64Images[i]}

			// 针对每个模型并发调用，简单限流：最多并发 4
			var mu sync.Mutex
			var mwg sync.WaitGroup
			sem := make(chan struct{}, 4)
			entry.ModelAnswers = make([]ModelAnswer, 0, len(models))

			for _, m := range models {
				m := m
				mwg.Add(1)
				go func() {
					defer mwg.Done()
					select {
					case sem <- struct{}{}:
						// ok
					case <-ctx.Done():
						return
					}
					defer func() { <-sem }()

					ans := a.callVision(ctx, m, base64Images[i])
					mu.Lock()
					entry.ModelAnswers = append(entry.ModelAnswers, ans)
					mu.Unlock()
				}()
			}
			mwg.Wait()
			items[i] = entry
		}()
	}
	wg.Wait()
	return items
}

// callVision 调用 SiliconFlow 兼容的 chat.completions（多模态），并尝试解析为问/答。
func (a *App) callVision(ctx context.Context, model, b64 string) ModelAnswer {
	baseURL := strings.TrimSpace(a.cfg.SiliconflowBaseURL)
	apiKey := strings.TrimSpace(a.cfg.SiliconflowAPIKey)
	result := ModelAnswer{Model: model}
	if apiKey == "" {
		result.Error = "缺少 API Key（请在 config.json 的 siliconflow_api_key 配置中设置）"
		return result
	}

	// OpenAI 风格的多模态消息结构
	userContent := []interface{}{
		map[string]interface{}{"type": "text", "text": promptText()},
		map[string]interface{}{
			"type":      "image_url",
			"image_url": map[string]interface{}{"url": "data:image/png;base64," + b64},
		},
	}

	// 构造请求体
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{"role": "system", "content": systemPrompt()},
			{"role": "user", "content": userContent},
		},
		"temperature": 0.2,
		"max_tokens":  800,
	}

	// 发起 HTTP 请求
	endpoint := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		result.Error = fmt.Sprintf("序列化请求失败: %v", err)
		return result
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		result.Error = fmt.Sprintf("构造请求失败: %v", err)
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("请求失败: %v", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(b))
		return result
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("读取响应失败: %v", err)
		return result
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		result.Error = fmt.Sprintf("解析响应失败: %v; 原始: %s", err, truncate(string(b), 500))
		return result
	}
	if len(parsed.Choices) == 0 {
		result.Error = "响应为空"
		return result
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	result.Raw = content

	// 解析 JSON 中的题目/答案（避免与接收者 a 冲突）
	q, ansText, ok := parseQA(content)
	if ok {
		result.Question = q
		result.Answer = ansText
	} else {
		// 若无法解析，作为降级：整段文本粗分
		result.Question, result.Answer = roughSplitQA(content)
	}
	return result
}

func systemPrompt() string {
	return "你是严格的题目解析助手。严格输出 JSON 格式，不添加任何额外文字、前缀或解释。若图片非题目，请保持 question 为空字符串，answer 填写 \"非题目\"。"
}

func promptText() string {
	return "从图片中抽取题目原文与标准答案，严格返回 JSON：{\"question\":\"...\",\"answer\":\"...\"}。注意：不要输出任何说明、标题、模型名、Markdown、代码块或多余文本；不要在字段中加入‘题目：’、‘答案：’等前缀。若无法确定题目则 question 为空字符串。"
}

// parseQA 尝试从模型文本中提取 JSON 并解析出 question/answer。
func parseQA(s string) (string, string, bool) {
	// 尝试直接解析
	type qa struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
		TiMu     string `json:"题目"`
		DaAn     string `json:"答案"`
	}
	var out qa
	if json.Unmarshal([]byte(s), &out) == nil {
		q := out.Question
		if q == "" {
			q = out.TiMu
		}
		a := out.Answer
		if a == "" {
			a = out.DaAn
		}
		if q != "" || a != "" {
			return q, a, true
		}
	}
	// 寻找首尾大括号尝试解析
	i := strings.IndexByte(s, '{')
	j := strings.LastIndexByte(s, '}')
	if i >= 0 && j > i {
		var out2 qa
		if json.Unmarshal([]byte(s[i:j+1]), &out2) == nil {
			q := out2.Question
			if q == "" {
				q = out2.TiMu
			}
			a := out2.Answer
			if a == "" {
				a = out2.DaAn
			}
			if q != "" || a != "" {
				return q, a, true
			}
		}
	}
	return "", "", false
}

// 粗略切分：按“题目/答案”或中文标记猜测
func roughSplitQA(s string) (string, string) {
	s = strings.TrimSpace(s)
	// 常见分隔
	for _, sep := range []string{"答案：", "答：", "参考答案：", "\n答案", "\n答"} {
		if idx := strings.Index(s, sep); idx > 0 {
			return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+len(sep):])
		}
	}
	return s, ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
