package ccguard

import (
	"regexp"
	"strings"
)

// 预编译正则表达式，避免每次调用时重复编译
var (
	ansiRegex        = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07`)
	boxCharsRegex    = regexp.MustCompile(`[─│┌┐└┘├┤┬┴┼╭╮╯╰]`)
	animationRegex   = regexp.MustCompile(`^[✻✶✳✢·✽⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]\s+\S+…`)
	shortcutsRegex   = regexp.MustCompile(`^\?\s+for\s+shortcuts`)
	multiNewlineRegex = regexp.MustCompile(`\n{3,}`)
	modelPatternRegex = regexp.MustCompile(`(\d+)\.\s+(\S+)`)
)

// OutputCleaner 输出清理器
type OutputCleaner struct{}

// NewOutputCleaner 创建输出清理器
func NewOutputCleaner() *OutputCleaner {
	return &OutputCleaner{}
}

// StripANSI 移除ANSI转义序列
func (c *OutputCleaner) StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// Clean 清理输出，移除ANSI序列、动画帧和多余空白
func (c *OutputCleaner) Clean(s string) string {
	s = c.StripANSI(s)
	// 移除box drawing字符
	s = boxCharsRegex.ReplaceAllString(s, "")

	// 按行分割，去重并过滤动画帧
	lines := strings.Split(s, "\n")
	seen := make(map[string]bool)
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// 跳过动画帧
		if animationRegex.MatchString(trimmed) {
			continue
		}

		// 跳过重复的 shortcuts 提示
		if shortcutsRegex.MatchString(trimmed) {
			if seen["shortcuts"] {
				continue
			}
			seen["shortcuts"] = true
		}

		// 跳过重复行（基于内容hash）
		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true

		result = append(result, trimmed)
	}

	// 压缩连续空白行
	output := strings.Join(result, "\n")
	output = multiNewlineRegex.ReplaceAllString(output, "\n\n")
	return strings.TrimSpace(output)
}

// 全局清理器实例
var defaultCleaner = NewOutputCleaner()

// StripANSI 全局函数，移除ANSI转义序列
func StripANSI(s string) string {
	return defaultCleaner.StripANSI(s)
}

// CleanOutput 全局函数，清理输出
func CleanOutput(s string) string {
	return defaultCleaner.Clean(s)
}

// SceneDetector 场景检测器
type SceneDetector struct{}

// NewSceneDetector 创建场景检测器
func NewSceneDetector() *SceneDetector {
	return &SceneDetector{}
}

// IsSelectScene 检测是否是选择场景（有导航提示）
func (d *SceneDetector) IsSelectScene(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(output, "↑/↓") ||
		strings.Contains(lower, "to navigate") ||
		strings.Contains(lower, "enter to select")
}

// HasNavigationHint 检测是否有导航提示
func (d *SceneDetector) HasNavigationHint(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "enter to select") ||
		strings.Contains(output, "↑/↓") ||
		strings.Contains(lower, "to navigate") ||
		strings.Contains(lower, "esc to cancel")
}

// HasNumberedOptions 检测是否有编号选项
func (d *SceneDetector) HasNumberedOptions(output string) bool {
	return strings.Contains(output, "1.") && strings.Contains(output, "2.")
}

// HasConfirmPrompt 检测是否有确认提示
func (d *SceneDetector) HasConfirmPrompt(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "do you want to proceed") ||
		strings.Contains(lower, "(y/n)") ||
		strings.Contains(lower, "[y/n]")
}

// NeedsDecision 检测输出是否需要进行决策判断
func (d *SceneDetector) NeedsDecision(output string) bool {
	if d.HasNumberedOptions(output) && d.HasNavigationHint(output) {
		return true
	}
	if d.HasConfirmPrompt(output) {
		return true
	}
	return false
}

// 全局场景检测器实例
var defaultSceneDetector = NewSceneDetector()

// IsSelectScene 全局函数，检测是否是选择场景
func IsSelectScene(output string) bool {
	return defaultSceneDetector.IsSelectScene(output)
}

// NeedsDecision 全局函数，检测是否需要决策
func NeedsDecision(output string) bool {
	return defaultSceneDetector.NeedsDecision(output)
}
