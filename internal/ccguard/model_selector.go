package ccguard

import (
	"io"
	"strings"
)

// ModelSelector 模型选择器，统一处理模型自动选择逻辑
type ModelSelector struct {
	modelName string
	selected  bool
}

// NewModelSelector 创建模型选择器
func NewModelSelector(modelName string) *ModelSelector {
	return &ModelSelector{
		modelName: modelName,
		selected:  false,
	}
}

// IsConfigured 检查是否配置了模型
func (m *ModelSelector) IsConfigured() bool {
	return m.modelName != ""
}

// IsSelected 检查模型是否已选择
func (m *ModelSelector) IsSelected() bool {
	return m.selected
}

// MarkSelected 标记模型已选择
func (m *ModelSelector) MarkSelected() {
	m.selected = true
}

// Reset 重置选择状态
func (m *ModelSelector) Reset() {
	m.selected = false
}

// FindModelNumber 从输出中查找配置的模型对应的编号
func (m *ModelSelector) FindModelNumber(output string) string {
	if m.modelName == "" {
		return ""
	}

	matches := modelPatternRegex.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			number := match[1]
			modelName := match[2]
			if strings.EqualFold(modelName, m.modelName) {
				return number
			}
		}
	}
	return ""
}

// NeedsSelection 检测输出是否包含模型选择提示
func (m *ModelSelector) NeedsSelection(output string) bool {
	return strings.Contains(output, "Please select a model")
}

// SelectModel 尝试自动选择模型，返回是否成功
// writer 用于发送选择输入（如 PTY）
func (m *ModelSelector) SelectModel(writer io.Writer, output string) bool {
	if m.selected || m.modelName == "" {
		return false
	}

	if !m.NeedsSelection(output) {
		return false
	}

	modelNum := m.FindModelNumber(output)
	if modelNum == "" {
		return false
	}

	// ccr 的模型选择需要回车确认
	_, err := writer.Write([]byte(modelNum + "\n"))
	if err != nil {
		DebugLog("ModelSelector: 写入模型选择失败: %v", err)
		return false
	}

	m.selected = true
	DebugLog("ModelSelector: 自动选择模型编号 %s", modelNum)
	return true
}

// GetModelName 获取配置的模型名称
func (m *ModelSelector) GetModelName() string {
	return m.modelName
}
