package merger

import (
	"sort"

	"xlsxtojson/schema"
)

// ClassData 合并后的一个 Class 完整数据
type ClassData struct {
	ClassName   string
	SheetData   []*SheetRows // 每个 Sheet 的数据和字段定义
	ParsedRows  []map[string]interface{} // 解析后的数据行（最终输出）
}

// SheetRows 每个 Sheet 的数据
type SheetRows struct {
	Schema *schema.SheetSchema // 该 Sheet 的字段定义
	Rows   [][]string         // 该 Sheet 的数据行
}

// Merge 按 ClassName 分组，将多 Sheet 数据行合并
func Merge(schemas []*schema.SheetSchema) (map[string]*ClassData, error) {
	classMap := make(map[string]*ClassData)

	for _, s := range schemas {
		className := s.ClassName

		if existing, ok := classMap[className]; ok {
			// 添加新的 Sheet 数据（保留各自的字段定义）
			existing.SheetData = append(existing.SheetData, &SheetRows{
				Schema: s,
				Rows:   s.DataRows,
			})
		} else {
			// 新建 ClassData
			classMap[className] = &ClassData{
				ClassName: className,
				SheetData: []*SheetRows{
					{
						Schema: s,
						Rows:   s.DataRows,
					},
				},
			}
		}
	}

	return classMap, nil
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

// FindPKColIndex 查找主键字段的实际列索引
func FindPKColIndex(fields []schema.FieldDef, pkName string) int {
	for _, f := range fields {
		if f.FieldName == pkName {
			return f.ColIndex
		}
	}
	return -1
}

// SortRowsByRows 按主键升序排列数据行（使用字段定义的列索引）
func SortRowsByRows(rows [][]string, fields []schema.FieldDef, pkName string) {
	pkColIndex := FindPKColIndex(fields, pkName)
	if pkColIndex < 0 {
		return
	}

	sort.Slice(rows, func(i, j int) bool {
		// 获取主键值
		var valI, valJ string
		if pkColIndex < len(rows[i]) {
			valI = rows[i][pkColIndex]
		}
		if pkColIndex < len(rows[j]) {
			valJ = rows[j][pkColIndex]
		}
		return valI < valJ
	})
}
