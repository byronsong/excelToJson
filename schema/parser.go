package schema

import (
	"errors"
	"fmt"
	"strings"
)

// ErrEmptyClassName 表示 A1 为空
var ErrEmptyClassName = errors.New("A1 为空")

// ErrNoFieldNameRow 表示未找到 FieldName 行
var ErrNoFieldNameRow = errors.New("未找到 FieldName 行")

// ErrNoTypeRow 表示未找到 Type 行
var ErrNoTypeRow = errors.New("未找到 Type 行")

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
	ClassName    string
	Fields       []FieldDef
	DataRows     [][]string // 原始字符串数据（从FieldName行+1开始）
	DataStartRow int        // 数据起始行号（1-based，用于错误定位）
	FileName     string     // 来源文件名，用于错误定位
	SheetName    string     // 来源 Sheet 名，用于错误定位
}

// ParseHeader 解析表头
// 通过查找 A 列值为 "Server" 的行来确定 FieldName 位置
// 通过查找 A 列值为 "Type" 的行来确定 Type 行位置
// 数据行从 FieldName 行 + 1 开始
func ParseHeader(rows [][]string, fileName, sheetName string) (*SheetSchema, error) {
	if len(rows) < 3 {
		return nil, fmt.Errorf("%s / %s: 表头行数不足，至少需要3行", fileName, sheetName)
	}

	// 查找 A 列值为 "Server" 的行（FieldName 行）
	fieldNameRowIdx := -1
	for i, row := range rows {
		if len(row) > 0 && strings.TrimSpace(row[0]) == ServerMarker {
			fieldNameRowIdx = i
			break
		}
	}

	if fieldNameRowIdx < 0 {
		return nil, fmt.Errorf("%s / %s: %w，跳过该 Sheet", fileName, sheetName, ErrNoFieldNameRow)
	}

	// 查找 A 列值为 "Type" 的行（Type 行）
	typeRowIdx := -1
	for i, row := range rows {
		if len(row) > 0 && strings.TrimSpace(row[0]) == TypeMarker {
			typeRowIdx = i
			break
		}
	}

	// 如果未找到 Type 行，但 FieldName 行位置合理，则使用 FieldName 行的前一行作为 Type 行（向后兼容）
	if typeRowIdx < 0 {
		if fieldNameRowIdx >= 1 {
			typeRowIdx = fieldNameRowIdx - 1
		} else {
			return nil, fmt.Errorf("%s / %s: %w，跳过该 Sheet", fileName, sheetName, ErrNoTypeRow)
		}
	}

	// 校验：需要至少3行（Label, Type, FieldName）
	if fieldNameRowIdx < 2 {
		return nil, fmt.Errorf("%s / %s: FieldName 行位置不对，需要至少3行表头", fileName, sheetName)
	}

	// 表头结构：
	// rows[0]: A列=ClassName, B列开始=中文标签
	// rows[typeRowIdx]: A列="Type", B列开始=类型
	// rows[fieldNameRowIdx]: A列="Server", B列开始=字段名
	// 数据从 fieldNameRowIdx + 1 开始
	nameRow := rows[fieldNameRowIdx] // Server行：字段名
	typeRow := rows[typeRowIdx]     // Type行：类型
	labelRow := rows[0]             // 第1行：中文标签

	// 从 A1 单元格获取 ClassName
	className := ""
	if len(rows[0]) > 0 && rows[0][0] != "" {
		className = strings.TrimSpace(rows[0][0])
	}

	if className == "" {
		return nil, fmt.Errorf("%s / %s: %w，跳过该 Sheet", fileName, sheetName, ErrEmptyClassName)
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
			if !IsNestedField(fieldName) {
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

	// 数据行从 FieldName 行 + 1 开始
	var dataRows [][]string
	if len(rows) > fieldNameRowIdx+1 {
		dataRows = rows[fieldNameRowIdx+1:]
	}

	return &SheetSchema{
		ClassName:    className,
		Fields:       fields,
		DataRows:     dataRows,
		DataStartRow: fieldNameRowIdx + 2, // 1-based: FieldName行是fieldNameRowIdx+1，数据从下一行开始
		FileName:     fileName,
		SheetName:    sheetName,
	}, nil
}

// IsNestedField 判断是否为嵌套字段（包含 . 或 [ 或 {）
func IsNestedField(fieldName string) bool {
	return strings.Contains(fieldName, ".") ||
		strings.Contains(fieldName, "[") ||
		strings.Contains(fieldName, "{")
}
