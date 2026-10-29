package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/joho/godotenv"
)

const (
	maxToolOutput = 50000
	toolTimeout   = 120 * time.Second
)

var dangerousPatterns = []string{"rm -rf /", "sudo", "shutdown", "reboot", "> /dev/"}

type bashInput struct {
	Command string `json:"command"`
}

func main() {
	if err := godotenv.Overload(); err != nil && !os.IsNotExist(err) {
		fmt.Printf("load .env failed: %v\n", err)
		return
	}

	ctx := context.Background()

	wd, err := os.Getwd() //
	if err != nil {
		fmt.Printf("get cwd failed: %v\n", err)
		return
	}
	//2.
	cm, err := newDeepSeekModel(ctx)
	if err != nil {
		fmt.Printf("create chat model failed: %v\n", err)
		return
	}
	//3.加载工具
	chatModel, err := cm.WithTools([]*schema.ToolInfo{
		{
			Name: "bash",
			Desc: "Run a shell command.", //工具函数的描述
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{ // 定义工具输入参数的 JSON Schema，模型按此生成 arguments
				"command": { //模型生成 arguments 时的 JSON key
					Type:     schema.String,               // 参数类型，对应 JSON Schema 的 string
					Desc:     "The shell command to run.", //给模型看的工具描述,影响模型填参质量
					Required: true,                        //表示"command"是必填参数,否则不会发起tool call
				},
			}),
		},
	})
	if err != nil {
		fmt.Printf("bind tools failed: %v\n", err)
		return
	}
	//
	//4.系统提示词+用户提示词（用户输入）
	system := fmt.Sprintf("You are a coding agent at %s. Use bash to solve tasks. Act, don't explain.", wd)
	history := []*schema.Message{schema.SystemMessage(system)}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\033[36ms01 >> \033[0m")
		if !scanner.Scan() {
			break
		}
		query := strings.TrimSpace(scanner.Text())
		if query == "" || strings.EqualFold(query, "q") || strings.EqualFold(query, "exit") {
			break
		}

		history = append(history, schema.UserMessage(query))
		if err := agentLoop(ctx, chatModel, &history); err != nil {
			fmt.Printf("agent loop error: %v\n\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("read input failed: %v\n", err)
	}
}

func newDeepSeekModel(ctx context.Context) (*deepseek.ChatModel, error) {
	apiKey := strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("missing api key: set DEEPSEEK_API_KEY in .env")
	}
	return deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
		APIKey:    apiKey,
		BaseURL:   "https://api.deepseek.com", // 可省略，有默认值
		Model:     "deepseek-chat",
		MaxTokens: 8000,
	})
}

func agentLoop(ctx context.Context, chatModel model.ToolCallingChatModel, history *[]*schema.Message) error {
	for {
		resp, err := chatModel.Generate(ctx, *history)
		if err != nil {
			return err
		}

		*history = append(*history, resp)
		if len(resp.ToolCalls) == 0 {
			if strings.TrimSpace(resp.Content) != "" {
				fmt.Println(resp.Content)
			}
			fmt.Println()
			return nil
		}

		toolResults := make([]*schema.Message, 0, len(resp.ToolCalls))
		for _, tc := range resp.ToolCalls {
			if tc.Function.Name != "bash" {
				toolResults = append(toolResults, schema.ToolMessage("Error: Unsupported tool", tc.ID, schema.WithToolName(tc.Function.Name)))
				continue
			}

			var in bashInput
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &in); err != nil {
				toolResults = append(toolResults, schema.ToolMessage("Error: Invalid tool arguments", tc.ID, schema.WithToolName(tc.Function.Name)))
				continue
			}

			fmt.Printf("\033[33m$ %s\033[0m\n", in.Command)
			out := runBash(in.Command)
			preview := out
			if len(preview) > 200 {
				preview = preview[:200]
			}
			fmt.Println(preview)

			toolResults = append(toolResults, schema.ToolMessage(out, tc.ID, schema.WithToolName(tc.Function.Name)))
		}

		*history = append(*history, toolResults...)
	}
}

func runBash(command string) string { //command:命令
	for _, d := range dangerousPatterns {
		if strings.Contains(command, d) { //d 包含commad
			return "Error: Dangerous command blocked"
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" { //windows系统
		cmd = exec.CommandContext(ctx, "powershell", "-Command", command)
	} else { //mac/linux系统
		cmd = exec.CommandContext(ctx, "bash", "-lc", command)
	}

	wd, err := os.Getwd() //
	if err == nil {
		cmd.Dir = wd
	}

	out, err := cmd.CombinedOutput() //Combine:合并
	if ctx.Err() == context.DeadlineExceeded {
		return "Error: Timeout (120s)"
	}

	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "(no output)"
	}
	if err != nil && result == "" {
		result = err.Error()
	}
	if len(result) > maxToolOutput {
		result = result[:maxToolOutput]
	}

	return result
}

func Define() {

}
