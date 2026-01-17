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

// needsDecision 检测输出是否需要进行决策判断
func (j *Judge) needsDecision(output string) bool {
	if NeedsDecision(output) {
		DebugLog("needsDecision: 检测到需要决策的场景")
		return true
	}
	// 没有明显的交互提示，可能还在执行中
	return false
}

// Decide 进行决策
func (j *Judge) Decide(output string) (*Decision, error) {
	// 检测是否需要决策
	if !j.needsDecision(output) {
		DebugLog("Judge.Decide: 无需决策，返回 WAIT")
		return &Decision{Action: "WAIT"}, nil
	}

	DebugLog("Judge.Decide: 检测到需要决策，调用 AI 进行判断")
	prompt := j.buildPrompt(output)
	DebugLogOutput("Judge.Decide: AI 提示词", prompt)

	DebugLog("Judge.Decide: 启动命令: %s code -p <prompt>", j.config.CCRCommand)
	cmd := exec.Command(j.config.CCRCommand, "code", "-p", prompt)

	// 使用 PTY 来处理可能的模型选择交互
	ptmx, err := pty.Start(cmd)
	if err != nil {
		DebugLog("Judge.Decide: 启动 AI 进程失败: %v", err)
		return nil, fmt.Errorf("failed to start judge process: %w", err)
	}
	defer ptmx.Close()
	DebugLog("Judge.Decide: AI 进程已启动，等待响应...")

	var outputBuf bytes.Buffer
	done := make(chan error, 1)
	lastLogLen := 0

	// 为此次调用创建独立的 model selector
	localModelSelector := NewModelSelector(j.config.Model)

	// 后台读取输出
	go func() {
		buf := make([]byte, DefaultReadBufSize)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				outputBuf.Write(buf[:n])

				// 实时输出 AI 响应到 debug（每次有新内容时输出增量）
				currentOutput := outputBuf.String()
				if len(currentOutput) > lastLogLen {
					newContent := currentOutput[lastLogLen:]
					DebugLog("Judge.Decide: AI 输出(增量): %s", StripANSI(newContent))
					lastLogLen = len(currentOutput)
				}

				// 检测模型选择提示并自动输入（只执行一次）
				if !localModelSelector.IsSelected() && localModelSelector.IsConfigured() {
					if localModelSelector.SelectModel(ptmx, currentOutput) {
						DebugLog("Judge.Decide: AI 进程自动选择模型")
					}
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
	DebugLog("Judge.Decide: 等待 AI 响应完成（超时 %v）...", DefaultJudgeTimeout)
	select {
	case err := <-done:
		if err != nil {
			DebugLog("Judge.Decide: AI 读取错误: %v", err)
			return nil, fmt.Errorf("judge AI read error: %w", err)
		}
		DebugLog("Judge.Decide: AI 响应读取完成")
	case <-time.After(DefaultJudgeTimeout):
		DebugLog("Judge.Decide: AI 超时")
		cmd.Process.Kill()
		cmd.Wait() // 等待进程退出，避免僵尸进程
		return nil, fmt.Errorf("judge AI timeout")
	}

	// 等待进程退出
	cmd.Wait()
	DebugLog("Judge.Decide: AI 进程已退出")

	aiResponse := outputBuf.String()
	DebugLogOutput("Judge.Decide: AI 完整响应", aiResponse)

	decision, err := j.parseResponse(aiResponse)
	if err != nil {
		DebugLog("Judge.Decide: 解析响应失败: %v", err)
		return nil, err
	}
	DebugLog("Judge.Decide: 解析结果 - Action: %s, Content: %s", decision.Action, decision.Content)
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

	return fmt.Sprintf(`你是 Claude Code 守卫的判断 AI。请根据用户任务和策略判断如何响应当前 CLI 输出。
%s
用户策略：
%s

主 Claude Code 进程最近的输出：
"""
%s
"""

你的任务是根据用户任务和策略自主决策。常见场景处理：

1. **选项选择**（显示编号列表如 1. xxx 2. xxx，带有 ↑/↓ 导航提示）
   - 根据用户任务和策略选择最合适的选项
   - 返回 SELECT:选项编号（如 SELECT:1 或 SELECT:2）
   - 注意：选项选择只需要按数字键，不需要回车

2. **确认提示**（Do you want to proceed? / Yes/No）
   - 根据策略判断操作是否安全且符合用户任务
   - 安全操作：返回 SELECT:1（选择 Yes）
   - 不确定或危险：返回 HUMAN:原因

3. **文本输入**（需要输入文字、路径、命令等）
   - 返回 INPUT:要输入的内容
   - 注意：文本输入需要回车确认

4. **危险操作识别**
   - 删除文件、修改系统配置、不可逆操作等
   - 根据策略判断：如果策略允许你自行判断，则判断后执行
   - 如果不确定是否有害，返回 HUMAN:原因

重要：优先根据用户任务和策略自主决策和行动，只有在策略明确要求人工介入或你确实无法判断时才返回 HUMAN。

请只回复一行，格式：
- SELECT:编号（选择菜单选项，只需按键不需回车）
- INPUT:内容（输入文本，需要回车确认）
- HUMAN:原因（需要人工介入，说明原因）

只回复一行，不要有其他内容。`, taskSection, j.config.Policy, cleanedOutput)
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

		if line == "WAIT" {
			DebugLog("parseResponse: 找到 WAIT (行 %d)", i+1)
			return &Decision{Action: "WAIT"}, nil
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

	// 没有找到有效决策
	DebugLog("parseResponse: 未找到有效决策，返回 WAIT")
	return &Decision{Action: "WAIT"}, nil
}
