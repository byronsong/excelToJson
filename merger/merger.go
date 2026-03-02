package merger

import (
	"fmt"
	"sort"

	"xlsxtojson/schema"
)

// ClassData 合并后的一个 Class 完整数据
type ClassData struct {
	ClassName   string
	Schema      *schema.SheetSchema // 字段定义以第一个 Sheet 为准
	Rows        [][]string          // 合并后的所有数据行（原始字符串）
	ParsedRows  []map[string]interface{} // 解析后的数据行
}

// Merge 按 ClassName 分组，将多 Sheet 数据行合并
func Merge(schemas []*schema.SheetSchema) (map[string]*ClassData, error) {
	classMap := make(map[string]*ClassData)

	for _, s := range schemas {
		className := s.ClassName

		if existing, ok := classMap[className]; ok {
			// 检查字段定义是否一致
			if !fieldsEqual(existing.Schema.Fields, s.Fields) {
				return nil, fmt.Errorf("%s: 字段定义冲突，Sheet '%s' 与之前定义的字段不一致",
					className, s.SheetName)
			}
			// 追加数据行
			existing.Rows = append(existing.Rows, s.DataRows...)
		} else {
			// 新建 ClassData
			classMap[className] = &ClassData{
				ClassName: className,
				Schema:    s,
				Rows:      s.DataRows,
			}
		}
	}

	return classMap, nil
}

// fieldsEqual 比较两个字段列表是否完全一致
func fieldsEqual(fields1, fields2 []schema.FieldDef) bool {
	if len(fields1) != len(fields2) {
		return false
	}
	for i := range fields1 {
		if fields1[i].FieldName != fields2[i].FieldName ||
			fields1[i].TypeStr != fields2[i].TypeStr {
			return false
		}
	}
	return true
}

// SortRows 按主键升序排列数据行
func SortRows(data *ClassData, pkIndex int) {
	if pkIndex < 0 {
		return
	}

	sort.Slice(data.Rows, func(i, j int) bool {
		// 获取主键值
		var valI, valJ string
		if pkIndex < len(data.Rows[i]) {
			valI = data.Rows[i][pkIndex]
		}
		if pkIndex < len(data.Rows[j]) {
			valJ = data.Rows[j][pkIndex]
		}
		return valI < valJ
	})
}

// FindPKIndex 查找主键字段的列索引
func FindPKIndex(fields []schema.FieldDef, pkName string) int {
	for i, f := range fields {
		if f.FieldName == pkName {
			return i
		}
	}
	return -1
}
