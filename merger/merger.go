package merger

import (
	"sort"

	"xlsxtojson/classconfig"
	"xlsxtojson/schema"
)

// ClassData 合并后的一个 Class 完整数据
type ClassData struct {
	ClassName   string
	Meta        *classconfig.ClassMeta // 来自 __ClassConfig，未声明时使用默认值
	SheetData   []*SheetRows // 每个 Sheet 的数据和字段定义
	ParsedRows  []map[string]interface{} // 解析后的数据行（最终输出）
}

// SheetRows 每个 Sheet 的数据
type SheetRows struct {
	Schema     *schema.SheetSchema // 该 Sheet 的字段定义
	Rows       [][]string         // 该 Sheet 的数据行
	SheetName  string            // Sheet 名称（用于 SheetName 注入）
}

// Merge 按 ClassName 分组，将多 Sheet 数据行合并
// classMetas 是从 __ClassConfig 解析出来的配置，key 是 className
func Merge(schemas []*schema.SheetSchema, classMetas map[string]*classconfig.ClassMeta) (map[string]*ClassData, error) {
	classMap := make(map[string]*ClassData)

	for _, s := range schemas {
		className := s.ClassName

		// 获取 ClassMeta，如果没有配置则使用默认值
		meta, ok := classMetas[className]
		if !ok {
			meta = classconfig.GetDefaultMeta(className)
		}

		if existing, ok := classMap[className]; ok {
			// 添加新的 Sheet 数据（保留各自的字段定义）
			existing.SheetData = append(existing.SheetData, &SheetRows{
				Schema:    s,
				Rows:      s.DataRows,
				SheetName: s.SheetName,
			})
		} else {
			// 新建 ClassData
			classMap[className] = &ClassData{
				ClassName: className,
				Meta:      meta,
				SheetData: []*SheetRows{
					{
						Schema:    s,
						Rows:      s.DataRows,
						SheetName: s.SheetName,
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
// 支持 single, composite, none 三种 pkType
func SortRowsByRows(rows [][]string, fields []schema.FieldDef, pkType classconfig.PkType, pkFields []string, sortFields []string) {
	switch pkType {
	case classconfig.PkTypeSingle:
		// 按单主键升序排序
		if len(pkFields) > 0 {
			pkColIndex := FindPKColIndex(fields, pkFields[0])
			if pkColIndex >= 0 {
				sort.Slice(rows, func(i, j int) bool {
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
		}

	case classconfig.PkTypeComposite:
		// 联合主键：保持 Excel 行顺序，不排序
		// 不做任何操作

	case classconfig.PkTypeNone:
		// 无主键模式
		if len(sortFields) > 0 {
			// 按 sortFields 升序排序
			sort.Slice(rows, func(i, j int) bool {
				for _, sf := range sortFields {
					colIndex := FindPKColIndex(fields, sf)
					if colIndex < 0 {
						continue
					}
					var valI, valJ string
					if colIndex < len(rows[i]) {
						valI = rows[i][colIndex]
					}
					if colIndex < len(rows[j]) {
						valJ = rows[j][colIndex]
					}
					if valI != valJ {
						return valI < valJ
					}
				}
				return false
			})
		}
		// 如果没有 sortFields，保持原顺序（不做操作）
	}
}
