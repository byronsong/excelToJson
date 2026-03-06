package globalconfig

import "fmt"

// GlobalEntry 公共配置中的一条配置项
type GlobalEntry struct {
	ID       string      // B 列：唯一标识
	TypeStr  string      // C 列：类型声明（空字符串表示需自动推断）
	RawValue string      // D 列：原始字符串值
	Value    interface{} // 按 TypeStr 解析后的值
	RowIndex int         // 行索引，用于错误定位
	FileName string      // 来源文件名，用于错误定位
	SheetName string     // 来源 Sheet 名，用于错误定位
}

// GlobalData 公共配置的完整数据
type GlobalData struct {
	Entries []*GlobalEntry
}

// ErrorType 错误类型
type ErrorType int

const (
	ErrEmptyID ErrorType = iota // id 为空
	ErrIDDuplicate             // id 重复
	ErrTypeInvalid             // 类型非法
	ErrValueParseFailed        // 值解析失败
)

// GlobalError 全局配置错误
type GlobalError struct {
	ErrType     ErrorType
	FileName   string
	SheetName  string
	Row        int
	FirstRow   int      // 首次出现行号（用于 ErrIDDuplicate）
	Col        string
	Message    string
	RawValueStr string // 用于错误信息中的实际值
}

func (e *GlobalError) Error() string {
	location := fmt.Sprintf("%s / %s / 行%d / 列%s", e.FileName, e.SheetName, e.Row, e.Col)

	switch e.ErrType {
	case ErrEmptyID:
		return fmt.Sprintf("[ERROR] %s (id): id 不能为空", location)
	case ErrIDDuplicate:
		return fmt.Sprintf("[ERROR] %s (id): id 重复，值 \"%s\" 已在行%d出现", location, e.Message, e.FirstRow)
	case ErrTypeInvalid:
		return fmt.Sprintf("[ERROR] %s (type): 不支持的类型 \"%s\"", location, e.Message)
	case ErrValueParseFailed:
		return fmt.Sprintf("[ERROR] %s (value): 期望 %s 类型，实际值 \"%s\" 解析失败", location, e.Message, e.RawValueStr)
	default:
		return fmt.Sprintf("[ERROR] %s: %s", location, e.Message)
	}
}

// GlobalConfigSheetName GlobalConfig Sheet 的 A1 标记
const GlobalConfigSheetName = "!GlobalConfig"
