package classconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xuri/excelize/v2"
)

func TestParseClassConfig(t *testing.T) {
	createTestFile := func(rows [][]string) *excelize.File {
		f := excelize.NewFile()
		index, _ := f.NewSheet("__ClassConfig")
		f.SetActiveSheet(index)

		for rowIdx, row := range rows {
			for colIdx, val := range row {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
				f.SetCellValue("__ClassConfig", cell, val)
			}
		}
		return f
	}

	tests := []struct {
		name     string
		rows     [][]string
		expected map[string]*ClassMeta
		wantErr  bool
	}{
		{
			name: "单主键配置",
			rows: [][]string{
				{"__ClassConfig", "Class名称", "主键类型", "主键字段"},
				{"", "string", "string", "string"},
				{"", "className", "pkType", "pkFields"},
				{"", "ItemConfig", "single", "id"},
			},
			expected: map[string]*ClassMeta{
				"ItemConfig": {
					ClassName:     "ItemConfig",
					PkType:        PkTypeSingle,
					PkFields:      []string{"id"},
					SortFields:    []string{},
					SheetNameAs:   "",
					SheetNameType: "",
					SourceFile:    "test.xlsx",
				},
			},
		},
		{
			name: "联合主键配置",
			rows: [][]string{
				{"__ClassConfig", "Class名称", "主键类型", "主键字段"},
				{"", "string", "string", "string"},
				{"", "className", "pkType", "pkFields"},
				{"", "EquipSlot", "composite", "equipTypeId,slotId"},
			},
			expected: map[string]*ClassMeta{
				"EquipSlot": {
					ClassName:     "EquipSlot",
					PkType:        PkTypeComposite,
					PkFields:      []string{"equipTypeId", "slotId"},
					SortFields:    []string{},
					SheetNameAs:   "",
					SheetNameType: "",
					SourceFile:    "test.xlsx",
				},
			},
		},
		{
			name: "无主键+排序字段+SheetName注入",
			rows: [][]string{
				{"__ClassConfig", "Class名称", "主键类型", "主键字段", "排序字段", "注入字段", "注入类型"},
				{"", "string", "string", "string", "string", "string", "string"},
				{"", "className", "pkType", "pkFields", "sortFields", "sheetNameAs", "sheetNameType"},
				{"", "RandomPool", "none", "", "groupId", "groupId", "int"},
			},
			expected: map[string]*ClassMeta{
				"RandomPool": {
					ClassName:     "RandomPool",
					PkType:        PkTypeNone,
					PkFields:      []string{},
					SortFields:    []string{"groupId"},
					SheetNameAs:   "groupId",
					SheetNameType: "int",
					SourceFile:    "test.xlsx",
				},
			},
		},
		{
			name: "缺少必填字段className",
			rows: [][]string{
				{"__ClassConfig", "主键类型"},
				{"", "string"},
				{"", "pkType"},
				{"", "single"},
			},
			wantErr: true,
		},
		{
			name: "无效的pkType",
			rows: [][]string{
				{"__ClassConfig", "Class名称", "主键类型"},
				{"", "string", "string"},
				{"", "className", "pkType"},
				{"", "Item", "invalid"},
			},
			wantErr: true,
		},
		{
			name: "single模式缺少pkFields",
			rows: [][]string{
				{"__ClassConfig", "Class名称", "主键类型", "主键字段"},
				{"", "string", "string", "string"},
				{"", "className", "pkType", "pkFields"},
				{"", "Item", "single", ""},
			},
			wantErr: true,
		},
		{
			name: "sheetNameAs和sheetNameType不匹配",
			rows: [][]string{
				{"__ClassConfig", "Class名称", "主键类型", "主键字段", "", "注入字段", "注入类型"},
				{"", "string", "string", "string", "", "string", "string"},
				{"", "className", "pkType", "pkFields", "", "sheetNameAs", "sheetNameType"},
				{"", "Item", "none", "", "", "groupId", ""},
			},
			wantErr: true,
		},
		{
			name: "无__ClassConfig Sheet",
			rows:     [][]string{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f *excelize.File
			if len(tt.rows) > 0 {
				f = createTestFile(tt.rows)
			} else {
				f = excelize.NewFile()
			}
			defer f.Close()

			result, err := ParseClassConfig(f, "test.xlsx")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
