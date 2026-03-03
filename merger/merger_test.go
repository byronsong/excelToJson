package merger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"xlsxtojson/classconfig"
	"xlsxtojson/schema"
)

func TestSortRowsByRows(t *testing.T) {
	tests := []struct {
		name       string
		rows       [][]string
		fields     []schema.FieldDef
		pkType     classconfig.PkType
		pkFields   []string
		sortFields []string
		expected   [][]string
	}{
		{
			name:   "single模式-数值排序",
			rows:   [][]string{{"10"}, {"2"}, {"1"}, {"5"}},
			fields: []schema.FieldDef{{FieldName: "id", ColIndex: 0, FieldType: schema.TypeInt}},
			pkType: classconfig.PkTypeSingle,
			pkFields: []string{"id"},
			// 修复后应为 ["1", "2", "5", "10"]
			expected: [][]string{{"1"}, {"2"}, {"5"}, {"10"}},
		},
		{
			name:   "single模式-字符串排序",
			rows:   [][]string{{"banana"}, {"apple"}, {"cherry"}},
			fields: []schema.FieldDef{{FieldName: "name", ColIndex: 0, FieldType: schema.TypeString}},
			pkType: classconfig.PkTypeSingle,
			pkFields: []string{"name"},
			expected: [][]string{{"apple"}, {"banana"}, {"cherry"}},
		},
		{
			name:   "composite模式-保持原序",
			rows:   [][]string{{"3"}, {"1"}, {"2"}},
			fields: []schema.FieldDef{{FieldName: "id", ColIndex: 0}},
			pkType: classconfig.PkTypeComposite,
			pkFields: []string{"id"},
			expected: [][]string{{"3"}, {"1"}, {"2"}},
		},
		{
			name:   "none模式-无sortFields-保持原序",
			rows:   [][]string{{"3"}, {"1"}, {"2"}},
			fields: []schema.FieldDef{{FieldName: "id", ColIndex: 0}},
			pkType: classconfig.PkTypeNone,
			expected: [][]string{{"3"}, {"1"}, {"2"}},
		},
		{
			name:       "none模式-有sortFields-排序",
			rows:       [][]string{{"b", "2"}, {"a", "1"}, {"c", "3"}},
			fields: []schema.FieldDef{
				{FieldName: "name", ColIndex: 0, FieldType: schema.TypeString},
				{FieldName: "order", ColIndex: 1, FieldType: schema.TypeInt},
			},
			pkType:     classconfig.PkTypeNone,
			sortFields: []string{"order"},
			expected:   [][]string{{"a", "1"}, {"b", "2"}, {"c", "3"}},
		},
		{
			name:       "none模式-多sortFields",
			rows:       [][]string{{"b", "2"}, {"a", "1"}, {"b", "1"}},
			fields: []schema.FieldDef{
				{FieldName: "name", ColIndex: 0, FieldType: schema.TypeString},
				{FieldName: "order", ColIndex: 1, FieldType: schema.TypeInt},
			},
			pkType:     classconfig.PkTypeNone,
			sortFields: []string{"name", "order"},
			expected:   [][]string{{"a", "1"}, {"b", "1"}, {"b", "2"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := make([][]string, len(tt.rows))
			copy(rows, tt.rows)

			SortRowsByRows(rows, tt.fields, tt.pkType, tt.pkFields, tt.sortFields)
			assert.Equal(t, tt.expected, rows)
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name       string
		schemas    []*schema.SheetSchema
		classMetas map[string]*classconfig.ClassMeta
		expected   int // 期望的 Class 数量
	}{
		{
			name: "单Schema",
			schemas: []*schema.SheetSchema{
				{ClassName: "Item"},
			},
			expected: 1,
		},
		{
			name: "多Schema同名合并",
			schemas: []*schema.SheetSchema{
				{ClassName: "Item"},
				{ClassName: "Item"},
			},
			expected: 1,
		},
		{
			name: "不同ClassName",
			schemas: []*schema.SheetSchema{
				{ClassName: "Item"},
				{ClassName: "Task"},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Merge(tt.schemas, tt.classMetas)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, len(result))
		})
	}
}
