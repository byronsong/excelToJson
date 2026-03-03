package classconfig

// PkType 主键类型枚举
type PkType string

const (
	PkTypeSingle    PkType = "single"
	PkTypeComposite PkType = "composite"
	PkTypeNone      PkType = "none"
)

// ClassMeta 一个 Class 的元配置（来自 __ClassConfig）
type ClassMeta struct {
	ClassName     string   // Class 名称
	PkType        PkType  // 主键类型
	PkFields      []string // 主键字段名列表（single 时长度为1，composite 时长度>=2）
	SortFields    []string // 排序字段（none 模式下使用）
	SheetNameAs   string   // 将 SheetName 注入为该字段名（空字符串表示不注入）
	SheetNameType string   // SheetNameAs 对应的类型声明（如 "int"、"string"）
}

// GetDefaultMeta 获取默认的 ClassMeta（当没有 __ClassConfig 时使用）
func GetDefaultMeta(className string) *ClassMeta {
	return &ClassMeta{
		ClassName:     className,
		PkType:        PkTypeSingle,
		PkFields:      []string{"id"},
		SortFields:    nil,
		SheetNameAs:   "",
		SheetNameType: "",
	}
}

// GetPKFieldName 获取主键字段名（single 模式返回第一个字段）
func (m *ClassMeta) GetPKFieldName() string {
	if len(m.PkFields) > 0 {
		return m.PkFields[0]
	}
	return "id"
}
