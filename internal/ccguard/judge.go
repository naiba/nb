package ccguard

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
)

// Decision 决策结果
type Decision struct {
	Action  string // WAIT, SELECT, INPUT, HUMAN
	Content string // SELECT/INPUT 的内容或 HUMAN 的原因
}

// Judge AI 决策器
type Judge struct {
	config *Config
	task   string // 用户传入的任务描述
}

// NewJudge 创建决策器
func NewJudge(config *Config, task string) *Judge {
	return &Judge{
		config: config,
		task:   task,
	}
}

// 询问计数器
var queryCounter int

// Decide 进行决策
// 当输出停止后被调用，AI负责判断是否需要行动
func (j *Judge) Decide(output string) (*Decision, error) {
	queryCounter++
	queryID := queryCounter

	prompt := j.buildPrompt(output)

	// 记录完整的 AI 询问内容
	DebugLog("========== 询问 AI #%d 开始 ==========", queryID)
	DebugLogOutput("提示词", prompt)

	cmd := exec.Command(j.config.CCRCommand, "code", "-p", prompt)

	// 使用 PTY 来处理可能的模型选择交互
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start judge process: %w", err)
	}
	defer ptmx.Close()

	var outputBuf bytes.Buffer
	done := make(chan error, 1)

	// 为此次调用创建独立的 model selector
	localModelSelector := NewModelSelector(j.config.Model)

	// 后台读取输出
	go func() {
		buf := make([]byte, DefaultReadBufSize)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				outputBuf.Write(buf[:n])

				// 检测模型选择提示并自动输入（只执行一次）
				if !localModelSelector.IsSelected() && localModelSelector.IsConfigured() {
					currentOutput := outputBuf.String()
					localModelSelector.SelectModel(ptmx, currentOutput)
				}
			}
			if err != nil {
				if err != io.EOF {
					done <- err
				} else {
					done <- nil
				}
				return
			}
		}
	}()

	// 等待进程完成，设置超时
	select {
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("judge AI read error: %w", err)
		}
	case <-time.After(DefaultJudgeTimeout):
		cmd.Process.Kill()
		cmd.Wait()
		return nil, fmt.Errorf("judge AI timeout")
	}

	cmd.Wait()

	aiResponse := outputBuf.String()

	// 记录 AI 响应
	DebugLogOutput("AI 响应", StripANSI(aiResponse))

	decision, err := j.parseResponse(aiResponse)
	if err != nil {
		return nil, err
	}

	DebugLog("AI 决策: %s %s", decision.Action, decision.Content)
	DebugLog("========== 询问 AI #%d 结束 ==========", queryID)
	return decision, nil
}

func (j *Judge) buildPrompt(output string) string {
	// 清理输出中的ANSI转义序列和特殊字符
	cleanedOutput := CleanOutput(output)

	// 构建任务描述部分
	taskSection := ""
	if j.task != "" {
		taskSection = fmt.Sprintf(`
用户正在执行的任务：
%s
`, j.task)
	}

	return fmt.Sprintf(`你是 Claude Code 守卫的判断 AI。当主 Claude Code 进程的输出停止变化后，你需要判断是否需要进行交互操作。
%s
用户策略：
%s

主 Claude Code 进程最近的输出：
"""
%s
"""

**首先判断：当前是否需要用户/你的操作？**

不需要操作的情况（返回 NONE）：
- AI 正在思考或执行任务中（输出显示 AI 正在工作，如代码编写、分析等）
- 输出是 AI 的回复或总结，不需要用户响应
- 没有明显的交互提示（如选项、确认框、输入提示等）
- 输出只是日志、进度信息等

需要操作的情况：
1. **选项选择**（显示编号列表如 1. xxx 2. xxx，或带有 ↑/↓ 导航提示）
   - 返回 SELECT:选项编号（如 SELECT:1 或 SELECT:2）

2. **确认提示**（Do you want to proceed? / Yes/No / y/n）
   - 安全操作：返回 SELECT:1（选择 Yes）
   - 不确定或危险：返回 HUMAN:原因

3. **文本输入**（显示输入提示符 ❯，且有问题等待回答）
   - 返回 INPUT:要输入的内容

4. **输入框残留**（看到 ❯ 后面跟着残留字符，如 "❯ 1"）
   - 返回 CLEAR 清理输入框

5. **危险操作**
   - 删除文件、修改系统配置等不可逆操作
   - 返回 HUMAN:原因

6. **无法理解的交互**
   - 返回 HUMAN:原因

**你可以使用的能力：**
- NONE：无操作
- WAIT：等待更多输出
- CLEAR：清理输入框中的残留字符（发送退格键）
- SELECT:编号：发送数字键选择菜单选项（不会按回车）
- INPUT:内容：输入文本内容并按回车确认
- HUMAN:原因：请求人工介入

请只回复一行，不要有其他内容。`, taskSection, j.config.Policy, cleanedOutput)
}

func (j *Judge) parseResponse(response string) (*Decision, error) {
	// 先清理 ANSI 序列
	response = StripANSI(response)
	response = strings.TrimSpace(response)
	lines := strings.Split(response, "\n")

	// 从后往前遍历所有行，找到第一个有效决策行
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		if line == "NONE" {
			DebugLog("parseResponse: 找到 NONE (行 %d)", i+1)
			return &Decision{Action: "NONE"}, nil
		}

		if line == "WAIT" {
			DebugLog("parseResponse: 找到 WAIT (行 %d)", i+1)
			return &Decision{Action: "WAIT"}, nil
		}

		if line == "CLEAR" {
			DebugLog("parseResponse: 找到 CLEAR (行 %d)", i+1)
			return &Decision{Action: "CLEAR"}, nil
		}

		if content, ok := strings.CutPrefix(line, "SELECT:"); ok {
			DebugLog("parseResponse: 找到 SELECT:%s (行 %d)", content, i+1)
			return &Decision{Action: "SELECT", Content: content}, nil
		}

		if content, ok := strings.CutPrefix(line, "INPUT:"); ok {
			DebugLog("parseResponse: 找到 INPUT:%s (行 %d)", content, i+1)
			return &Decision{Action: "INPUT", Content: content}, nil
		}

		if reason, ok := strings.CutPrefix(line, "HUMAN:"); ok {
			DebugLog("parseResponse: 找到 HUMAN:%s (行 %d)", reason, i+1)
			return &Decision{Action: "HUMAN", Content: reason}, nil
		}
		// 继续往前找，不在第一个非空行就停止
	}

	// 没有找到有效决策，默认返回 NONE（不操作）
	DebugLog("parseResponse: 未找到有效决策，返回 NONE")
	return &Decision{Action: "NONE"}, nil
}
