package schema

import (
	"fmt"
	"strings"
)

// FieldDef 一个字段的元信息
type FieldDef struct {
	Label     string    // 中文含义（第1行）
	TypeStr   string    // 原始类型字符串（第2行）
	FieldName string    // 英文字段名（第3行）
	FieldType FieldType // 解析后的枚举类型
	ColIndex  int       // 列索引，用于错误定位
	Ignored   bool      // 是否忽略
}

// SheetSchema 一个 Sheet 解析出的元信息
type SheetSchema struct {
	ClassName string
	Fields    []FieldDef
	DataRows  [][]string // 原始字符串数据（第4行起）
	FileName  string     // 来源文件名，用于错误定位
	SheetName string     // 来源 Sheet 名，用于错误定位
}

// ParseHeader 解析表头
// rows[0] 是第1行（中文含义），rows[1] 是第2行（类型），rows[2] 是第3行（FieldName）
func ParseHeader(rows [][]string, fileName, sheetName string) (*SheetSchema, error) {
	if len(rows) < 4 {
		return nil, fmt.Errorf("%s / %s: 表头行数不足，至少需要4行", fileName, sheetName)
	}

	// 第1行是中文含义，第2行是类型，第3行是FieldName
	labelRow := rows[0]
	typeRow := rows[1]
	nameRow := rows[2]

	// 从 A1 单元格获取 ClassName（第1行第1列）
	className := ""
	if len(rows[0]) > 0 && rows[0][0] != "" {
		className = strings.TrimSpace(rows[0][0])
	}

	if className == "" {
		return nil, fmt.Errorf("%s / %s: A1 为空，跳过该 Sheet", fileName, sheetName)
	}

	// 解析字段定义（从 B 列开始，即 index 1）
	fields := make([]FieldDef, 0)
	for i := 1; i < len(typeRow); i++ {
		typeStr := strings.TrimSpace(typeRow[i])
		fieldName := ""
		if i < len(nameRow) {
			fieldName = strings.TrimSpace(nameRow[i])
		}
		label := ""
		if i < len(labelRow) {
			label = strings.TrimSpace(labelRow[i])
		}

		// 跳过空字段
		if typeStr == "" && fieldName == "" {
			continue
		}

		// 判断是否忽略列
		ignored := typeStr == "ignore" || strings.HasPrefix(fieldName, "#")

		fieldType := ParseFieldType(typeStr)
		if typeStr != "" && fieldType == TypeUnknown && !ignored {
			// 如果是嵌套字段（包含 . 或 [ 或 {），则不检查基础类型
			if !isNestedField(fieldName) {
				return nil, fmt.Errorf("%s / %s / 列%d (%s): 未知类型 '%s'",
					fileName, sheetName, i+1, fieldName, typeStr)
			}
		}

		fields = append(fields, FieldDef{
			Label:     label,
			TypeStr:   typeStr,
			FieldName: fieldName,
			FieldType: fieldType,
			ColIndex:  i,
			Ignored:   ignored,
		})
	}

	// 数据行从第4行开始（index 3）
	var dataRows [][]string
	if len(rows) > 3 {
		dataRows = rows[3:]
	}

	return &SheetSchema{
		ClassName: className,
		Fields:    fields,
		DataRows:  dataRows,
		FileName:  fileName,
		SheetName: sheetName,
	}, nil
}

// isNestedField 判断是否为嵌套字段（包含 . 或 [ 或 {）
func isNestedField(fieldName string) bool {
	return strings.Contains(fieldName, ".") ||
		strings.Contains(fieldName, "[") ||
		strings.Contains(fieldName, "{")
}
