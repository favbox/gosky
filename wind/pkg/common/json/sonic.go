//go:build (linux || windows || darwin) && amd64 && !stdjson

package json

import "github.com/bytedance/sonic"

// Name 是有效的JSON 包名。
const Name = "sonic"

var (
	json = sonic.ConfigStd
	// Marshal 用于渲染 JSON 而导出的标库准实现。
	Marshal = json.Marshal
	// Unmarshal 用于绑定 JSON 而导出的标准库实现。
	Unmarshal = json.Unmarshal
	// MarshalIndent 用于渲染带缩进格式的 JSON 而导出的标准库实现。
	MarshalIndent = json.MarshalIndent
	// NewDecoder 用于读取 io.Reader 而导出的 JSON 阅读器。
	NewDecoder = json.NewDecoder
	// NewEncoder 用于写入 io.Writer 而导出的 JSON 编码器。
	NewEncoder = json.NewEncoder
)
