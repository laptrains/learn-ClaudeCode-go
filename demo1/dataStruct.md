这段 tool schema 里几个关键字段的含义 + 为什么这么写 + 不写会怎样。会逐个拆开讲，讲完你就能自己随便加工具了。（方便学习或复习）

一、整体结构回顾

tools := []Tool{
    {
        Type: "function",
        Function: ToolDef{
            Name:        "bash",
            Description: "执行 shell 命令并返回输出",
            Parameters: Parameters{
                Type:       "object",
                Properties: map[string]Property{ ... },
                Required:   []string{"command"},
            },
        },
    },
}


这是 OpenAI / DeepSeek / 通义等主流 LLM 通用的 tool schema 格式，本质就是告诉模型：

"你有一个工具可以用，它长这样，参数长这样，哪些必填"

二、Type: "function" 是什么？

Type: "function"


✅ 当前只有这一种值，是 OpenAI 定的协议。

含义：告诉模型这是一个函数调用型工具。

未来可能会有：
• "type": "code_interpreter"

• "type": "retrieval"

• "type": "mcp"（新协议）

但现在你写 "function" 就行，别改。

三、Parameters 是什么？

Parameters: Parameters{
    Type: "object",
    ...
}


这是整个 tool 定义里最重要、也最容易写错的部分。

Type: "object" 是什么意思？

含义：  
模型生成的 arguments 必须是一个 JSON 对象

也就是模型必须生成这种格式：
{
  "command": "ls -l"
}


❌ 不能生成：
"ls -l"

❌ 不能生成：
["ls", "-l"]


如果你写成 Type: "string"，模型会懵，绝大多数 API 会直接报错或行为不可预期。

✅ tool 的 Parameters.Type 永远是 "object"

四、Properties 是什么？（核心）

Properties: map[string]Property{
    "command": {
        Type:        "string",
        Description: "要执行的 shell 命令",
    },
}


一句话解释

Properties 就是参数的字段定义表——告诉模型：

- 有哪些参数

- 每个参数什么类型

- 每个参数干什么用

逐个字段拆解

字段 含义 为什么重要

command 参数名 模型生成 JSON 时的 key

Type: "string" 参数类型 模型知道填字符串

Description 参数说明 模型知道填什么内容

模型看到这个定义后，会生成：

{
  "command": "ls -l"
}


然后你代码里：
var input struct {
    Command string `json:"command"`
}
json.Unmarshal([]byte(tc.Function.Arguments), &input)


input.Command 就拿到 "ls -l" 了。

五、Required 是什么？

Required: []string{"command"}


一句话解释

告诉模型：哪些参数是必填的

不写 Required 会怎样？

模型可能生成：
{}


或者：
{
  "command": ""
}


然后你的代码就炸了。

写了 Required 之后

模型知道：
• command 必须给

• 不给就不算完成 tool call

✅ 有必填参数就一定要写 Required

六、如果工具有多个参数呢？

比如你加一个 timeout 参数：
Properties: map[string]Property{
    "command": {
        Type:        "string",
        Description: "要执行的 shell 命令",
    },
    "timeout": {
        Type:        "integer",
        Description: "超时时间（秒）",
    },
},
Required: []string{"command"},


模型就可以生成：
{
  "command": "sleep 5",
  "timeout": 3
}


而 timeout 是可选的（因为不在 Required 里）。

七、支持的参数类型有哪些？

OpenAI / DeepSeek 兼容的 schema 支持：

JSON Schema 类型 Go 对应 说明

"string" string 文本

"integer" int 整数

"number" float64 浮点数

"boolean" bool true/false

"array" []T 数组

"object" struct 嵌套对象

示例：数组参数

"args": {
    Type:        "array",
    Description: "命令参数列表",
    Items: &Property{
        Type: "string",
    },
}


模型生成：
{
  "command": "ls",
  "args": ["-l", "-a"]
}


八、一个完整的多参数工具示例

{
    Type: "function",
    Function: ToolDef{
        Name:        "write_file",
        Description: "写入内容到指定文件",
        Parameters: Parameters{
            Type: "object",
            Properties: map[string]Property{
                "path": {
                    Type:        "string",
                    Description: "文件路径",
                },
                "content": {
                    Type:        "string",
                    Description: "要写入的内容",
                },
                "append": {
                    Type:        "boolean",
                    Description: "是否追加模式，默认 false",
                },
            },
            Required: []string{"path", "content"},
        },
    },
}


九、常见错误汇总 ⚠️

错误 后果

不写 Parameters 模型不知道参数格式，arguments 乱填

Type 不写 "object" API 报错或模型行为异常

Properties 里 key 和 Go struct 字段不一致 json.Unmarshal 拿不到值

Description 写得太模糊 模型传的参数质量差

必填参数不写 Required 模型可能漏传

Description 用中文但模型是英文 prompt 偶尔影响效果（不致命）

十、一句话记住

Properties = 参数清单，Required = 必填标记，Type: "object" = 告诉模型用 JSON 对象传参
