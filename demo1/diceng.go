// package main

// import (
// 	"bufio"
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"net/http"
// 	"os"
// 	"os/exec"
// 	"strings"
// 	"time"
// )

// //这个文件完全复刻main.go,没有引入框架，只用go标准库，适合理解底层（建议先结合AI弄懂数据结构）
// // ========== 1. 数据结构 ==========

// type ChatMessage struct {
// 	Role       string     `json:"role"`
// 	Content    string     `json:"content,omitempty"`      //omitempty：如果为空，JSON 里不出现这个字段
// 	ToolCallID string     `json:"tool_call_id,omitempty"` //工具结果关联到哪次调用
// 	Name       string     `json:"name,omitempty"`         //工具名字
// 	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   //模型请求调用工具（关键），只有 assistant 消息才会出现：模型告诉你：“我想调用这些工具，你去执行一下
// }

// type ToolCall struct {
// 	ID       string   `json:"id"`       //本次工具调用的唯一 ID，用于 tool 消息回关联
// 	Type     string   `json:"type"`     //固定 "function"/也可以是"mcp
// 	Function ToolFunc `json:"function"` //具体调用哪个函数、传什么参数
// }

// type ToolFunc struct {
// 	Name      string `json:"name"`      //工具名：必须和你tools 定义的一致
// 	Arguments string `json:"arguments"` //Json字符串（不是对象，需要反序列化进struct对象里）：必须和你 tools 定义里的 Name 一致
// }

// type ChatRequest struct {
// 	Model    string        `json:"model"`
// 	Messages []ChatMessage `json:"messages"`
// 	Tools    []Tool        `json:"tools,omitempty"`
// }

// type Tool struct {
// 	Type     string  `json:"type"` //例子：Type: "function"，告诉模型这是一个函数调用型工具
// 	Function ToolDef `json:"function"`
// }

// type ToolDef struct {
// 	Name        string     `json:"name"`
// 	Description string     `json:"description"`
// 	Parameters  Parameters `json:"parameters"`
// }

// type Parameters struct {
// 	Type       string              `json:"type"`       //模型必须生成一个Json对象
// 	Properties map[string]Property `json:"properties"` //给模型的参数字段定义表
// 	Required   []string            `json:"required"`   //require:必填参数
// }

// type Property struct {
// 	Type        string `json:"type"`
// 	Description string `json:"description"`
// }

// type ChatResponse struct {
// 	Choices []struct {
// 		Message ChatMessage `json:"message"`
// 	} `json:"choices"`
// }

// // ========== 2. 调模型 ==========

// func callDeepSeek(ctx context.Context, req ChatRequest) (*ChatMessage, error) {
// 	body, _ := json.Marshal(req)

// 	httpReq, _ := http.NewRequestWithContext(
// 		ctx,
// 		http.MethodPost,
// 		"https://api.deepseek.com/v1/chat/completions",
// 		strings.NewReader(string(body)),
// 	)
// 	httpReq.Header.Set("Content-Type", "application/json")
// 	httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))) //直接在bash里面输入apikey末尾会带一个\n

// 	client := &http.Client{Timeout: 60 * time.Second}
// 	resp, err := client.Do(httpReq)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()

// 	var chatResp ChatResponse
// 	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil { //:从resp.Body里读取，解析成Json，Decode把Json绑定到结构体里去
// 		return nil, err
// 	}
// 	if len(chatResp.Choices) == 0 {
// 		return nil, fmt.Errorf("empty response")
// 	}
// 	return &chatResp.Choices[0].Message, nil
// }

// // ========== 3. 执行 bash(示例工具) ==========

// func runBash(command string) string {
// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	cmd := exec.CommandContext(ctx, "bash", "-c", command)
// 	out, err := cmd.CombinedOutput()
// 	if err != nil {
// 		return fmt.Sprintf("error: %v\n%s", err, out)
// 	}
// 	return string(out)
// }

// // ========== 4. Agent Loop ==========

// func AgentLoop(ctx context.Context, history []ChatMessage) ([]ChatMessage, error) {
// 	tools := []Tool{
// 		{
// 			Type: "function",
// 			Function: ToolDef{
// 				Name:        "bash",
// 				Description: "执行 shell 命令并返回输出",
// 				Parameters: Parameters{
// 					Type: "object", //对应go的struct
// 					Properties: map[string]Property{ //Properties:核心参数结构，给模型使用的（要加参数也是在这里加）
// 						"command": {
// 							Type:        "string",
// 							Description: "要执行的 shell 命令",
// 						},
// 					},
// 					Required: []string{"command"}, //Required:必填参数
// 				},
// 			},
// 		},
// 	}

// 	for {
// 		req := ChatRequest{
// 			Model:    "deepseek-chat",
// 			Messages: history,
// 			Tools:    tools,
// 		}

// 		msg, err := callDeepSeek(ctx, req)
// 		if err != nil {
// 			return history, err
// 		}

// 		// 把 assistant 消息加入历史
// 		history = append(history, *msg)

// 		// 没有工具调用 → 输出最终回答
// 		if len(msg.ToolCalls) == 0 {
// 			if strings.TrimSpace(msg.Content) != "" {
// 				fmt.Println("\n🤖", msg.Content)
// 			}
// 			return history, nil
// 		}

// 		// 处理工具调用
// 		for _, tc := range msg.ToolCalls {
// 			fmt.Printf("\n🔧 调用工具 %s: %s\n", tc.Function.Name, tc.Function.Arguments)

// 			if tc.Function.Name != "bash" {
// 				history = append(history, ChatMessage{
// 					Role:       "tool",
// 					ToolCallID: tc.ID,
// 					Name:       tc.Function.Name,
// 					Content:    "未知工具",
// 				})
// 				continue
// 			}

// 			var input struct {
// 				Command string `json:"command"`
// 			}
// 			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
// 				history = append(history, ChatMessage{
// 					Role:       "tool",
// 					ToolCallID: tc.ID,
// 					Name:       "bash",
// 					Content:    "参数解析失败: " + err.Error(),
// 				})
// 				continue
// 			}

// 			out := runBash(input.Command)
// 			fmt.Println("📤 输出:", out)

// 			history = append(history, ChatMessage{
// 				Role:       "tool",
// 				ToolCallID: tc.ID,
// 				Name:       "bash",
// 				Content:    out,
// 			})
// 		}
// 	}
// }

// // ========== 5. main ==========

// func main() {
// 	if os.Getenv("DEEPSEEK_API_KEY") == "" {
// 		fmt.Println("请设置 DEEPSEEK_API_KEY 环境变量")
// 		return
// 	}

// 	wd, _ := os.Getwd()
// 	systemPrompt := fmt.Sprintf("你是一个 AI 助手，可以使用 bash 工具。工作目录: %s", wd)

// 	history := []ChatMessage{
// 		{Role: "system", Content: systemPrompt},
// 	}

// 	scanner := bufio.NewScanner(os.Stdin)
// 	fmt.Println("输入 exit 退出")

// 	for {
// 		fmt.Print("\n> ")
// 		if !scanner.Scan() {
// 			break
// 		}
// 		input := strings.TrimSpace(scanner.Text())
// 		if input == "" || input == "exit" {
// 			break
// 		}

// 		history = append(history, ChatMessage{
// 			Role:    "user",
// 			Content: input,
// 		})

// 		var err error
// 		history, err = AgentLoop(context.Background(), history)
// 		if err != nil {
// 			fmt.Println("❌ 错误:", err)
// 		}
// 	}
// }
