package ccguard

import (
	"regexp"
	"strings"
)

// 预编译正则表达式，避免每次调用时重复编译
var (
	ansiRegex         = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07`)
	boxCharsRegex     = regexp.MustCompile(`[─│┌┐└┘├┤┬┴┼╭╮╯╰]`)
	animationRegex    = regexp.MustCompile(`^[✻✶✳✢·✽⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]\s+\S+…`)
	shortcutsRegex    = regexp.MustCompile(`^\?\s+for\s+shortcuts`)
	multiNewlineRegex = regexp.MustCompile(`\n{3,}`)
	modelPatternRegex = regexp.MustCompile(`(\d+)\.\s+(\S+)`) // 用于解析模型选择列表
	// 输入框残留检测：❯ 后面跟着短残留内容（1-3个字符，通常是数字或字母）
	inputResidueRegex = regexp.MustCompile(`❯\s+([0-9]{1,2})$`)
	// HTML meta 标签清理：移除可能导致 shell 解析问题的 HTML 标签
	// 例如 <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1">
	// 中的 "initial-scale=1," 可能被 shell 误解析为命令
	htmlMetaRegex = regexp.MustCompile(`<meta[^>]*>`)
	// HTML 标签内的属性值，可能包含 shell 敏感字符
	htmlTagRegex = regexp.MustCompile(`<[a-zA-Z][^>]*>`)
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

// SanitizeForPrompt 清理输出以便安全地用于 AI prompt
// 移除可能导致 shell 解析问题的 HTML 标签（如 <meta viewport>）
// 保留文本内容以便 AI 理解上下文
func SanitizeForPrompt(s string) string {
	// 移除 HTML meta 标签（这些通常包含可能被 shell 误解析的属性）
	s = htmlMetaRegex.ReplaceAllString(s, "[HTML meta tag removed]")
	// 可选：移除其他 HTML 标签，但保留内容
	// s = htmlTagRegex.ReplaceAllString(s, "")
	return s
}

// SceneDetector 场景检测器（简化版，仅保留必要功能）
type SceneDetector struct{}

// NewSceneDetector 创建场景检测器
func NewSceneDetector() *SceneDetector {
	return &SceneDetector{}
}

// IsSelectScene 检测是否是选择场景（有导航提示）
// 用于判断输入后是否需要回车
func (d *SceneDetector) IsSelectScene(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(output, "↑/↓") ||
		strings.Contains(lower, "to navigate") ||
		strings.Contains(lower, "enter to select")
}

// InputResidueInfo 输入框残留信息
type InputResidueInfo struct {
	HasResidue bool   // 是否有残留
	Residue    string // 残留内容
}

// DetectInputResidue 检测输入框残留内容
// 检测 ❯ 后面是否有残留的数字（如选择后遗留的 "1" 或 "2"）
func (d *SceneDetector) DetectInputResidue(output string) InputResidueInfo {
	// 先清理 ANSI 转义序列（包括光标等）
	cleanedOutput := StripANSI(output)
	// 按行查找，只检查最后几行（输入框通常在底部）
	lines := strings.Split(cleanedOutput, "\n")
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-5; i-- {
		line := strings.TrimSpace(lines[i])
		if matches := inputResidueRegex.FindStringSubmatch(line); len(matches) > 1 {
			residue := strings.TrimSpace(matches[1])
			if residue != "" {
				return InputResidueInfo{
					HasResidue: true,
					Residue:    residue,
				}
			}
		}
	}
	return InputResidueInfo{HasResidue: false}
}

// HasInputPrompt 检测是否有输入框提示符（❯）
// 用于判断 SELECT 操作是否需要发送回车
func (d *SceneDetector) HasInputPrompt(output string) bool {
	cleanedOutput := StripANSI(output)
	lines := strings.Split(cleanedOutput, "\n")
	// 检查最后几行是否有输入提示符
	for i := len(lines) - 1; i >= 0 && i >= len(lines)-5; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(line, "❯") {
			return true
		}
	}
	return false
}

// 全局场景检测器实例
var defaultSceneDetector = NewSceneDetector()

// IsSelectScene 全局函数，检测是否是选择场景
func IsSelectScene(output string) bool {
	return defaultSceneDetector.IsSelectScene(output)
}

// HasInputPrompt 全局函数，检测是否有输入框
func HasInputPrompt(output string) bool {
	return defaultSceneDetector.HasInputPrompt(output)
}

// DetectInputResidue 全局函数，检测输入框残留
func DetectInputResidue(output string) InputResidueInfo {
	return defaultSceneDetector.DetectInputResidue(output)
}
