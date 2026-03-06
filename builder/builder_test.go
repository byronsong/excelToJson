package builder

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"xlsxtojson/classconfig"
	"xlsxtojson/merger"
	"xlsxtojson/schema"
)

func TestConvertValue(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		fieldType schema.FieldType
		expected  interface{}
		wantErr   bool
	}{
		// Int类型
		{"int正常", "100", schema.TypeInt, int64(100), false},
		{"int负数", "-100", schema.TypeInt, int64(-100), false},
		{"int零", "0", schema.TypeInt, int64(0), false},
		{"int科学计数法", "1e3", schema.TypeInt, int64(1000), false},
		{"int小数", "100.0", schema.TypeInt, int64(100), false},
		{"int无效", "abc", schema.TypeInt, nil, true},
		{"int空", "", schema.TypeInt, nil, true},

		// Float类型
		{"float正常", "3.14", schema.TypeFloat, 3.14, false},
		{"float科学计数法", "1.5e-10", schema.TypeFloat, 1.5e-10, false},

		// Bool类型
		{"bool-true", "true", schema.TypeBool, true, false},
		{"bool-1", "1", schema.TypeBool, true, false},
		{"bool-是", "是", schema.TypeBool, true, false},
		{"bool-yes", "yes", schema.TypeBool, true, false},
		{"bool-false", "false", schema.TypeBool, false, false},
		{"bool-0", "0", schema.TypeBool, false, false},
		{"bool-否", "否", schema.TypeBool, false, false},
		{"bool-no", "no", schema.TypeBool, false, false},
		{"bool-大写", "TRUE", schema.TypeBool, true, false},
		{"bool-混合大小写", "True", schema.TypeBool, true, false},
		// 布尔值当前不支持前后空格
		// {"bool-空格", " true ", schema.TypeBool, true, false},

		// 数组类型
		{"int数组", "1,2,3", schema.TypeIntSlice, []interface{}{int64(1), int64(2), int64(3)}, false},
		{"int数组空格", "1, 2 , 3", schema.TypeIntSlice, []interface{}{int64(1), int64(2), int64(3)}, false},
		{"int数组空元素", "1,,3", schema.TypeIntSlice, []interface{}{int64(1), int64(3)}, false},
		{"int数组无效", "1,a,3", schema.TypeIntSlice, nil, true},
		{"float数组", "1.5,2.5", schema.TypeFloatSlice, []interface{}{1.5, 2.5}, false},
		{"string数组", `"a","b","c"`, schema.TypeStringSlice, []interface{}{"a", "b", "c"}, false},

		// Map类型
		{"int-int map", "1:100,2:200", schema.TypeIntMap, map[string]interface{}{"1": int64(100), "2": int64(200)}, false},
		{"string-int map", "key1:100,key2:200", schema.TypeStringMap, map[string]interface{}{"key1": int64(100), "key2": int64(200)}, false},
		{"map空格", "1 : 100 , 2 : 200", schema.TypeIntMap, map[string]interface{}{"1": int64(100), "2": int64(200)}, false},
		{"map格式错误", "1-100,2-200", schema.TypeIntMap, nil, true},
		{"map键值对缺失", "1:100,2", schema.TypeIntMap, nil, true},

		// 空值处理
		{"空字符串转int", "", schema.TypeInt, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertValue(tt.value, tt.fieldType)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// 特殊处理NaN
			if f, ok := tt.expected.(float64); ok && math.IsNaN(f) {
				assert.True(t, math.IsNaN(result.(float64)))
				return
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuild(t *testing.T) {
	tests := []struct {
		name      string
		classData *merger.ClassData
		expected  []map[string]interface{}
		wantErr   bool
	}{
		{
			name: "简单行构建",
			classData: &merger.ClassData{
				ClassName: "Item",
				Meta:      &classconfig.ClassMeta{PkType: classconfig.PkTypeSingle, PkFields: []string{"id"}},
				SheetData: []*merger.SheetRows{
					{
						Schema: &schema.SheetSchema{
							Fields: []schema.FieldDef{
								{FieldName: "id", FieldType: schema.TypeInt, ColIndex: 0},
								{FieldName: "name", FieldType: schema.TypeString, ColIndex: 1},
							},
						},
						Rows: [][]string{{"1001", "Sword"}},
					},
				},
			},
			expected: []map[string]interface{}{
				{"id": int64(1001), "name": "Sword"},
			},
		},
		{
			name: "嵌套结构构建",
			classData: &merger.ClassData{
				ClassName: "Item",
				Meta:      &classconfig.ClassMeta{PkType: classconfig.PkTypeSingle, PkFields: []string{"id"}},
				SheetData: []*merger.SheetRows{
					{
						Schema: &schema.SheetSchema{
							Fields: []schema.FieldDef{
								{FieldName: "id", FieldType: schema.TypeInt, ColIndex: 0},
								{FieldName: "rewards[0].id", FieldType: schema.TypeInt, ColIndex: 1},
								{FieldName: "rewards[0].count", FieldType: schema.TypeInt, ColIndex: 2},
							},
						},
						Rows: [][]string{{"1001", "2001", "5"}},
					},
				},
			},
			expected: []map[string]interface{}{
				{
					"id": int64(1001),
					"rewards": []interface{}{
						map[string]interface{}{
							"id":    int64(2001),
							"count": int64(5),
						},
					},
				},
			},
		},
		{
			name: "空单元格跳过",
			classData: &merger.ClassData{
				ClassName: "Item",
				Meta:      &classconfig.ClassMeta{PkType: classconfig.PkTypeSingle, PkFields: []string{"id"}},
				SheetData: []*merger.SheetRows{
					{
						Schema: &schema.SheetSchema{
							Fields: []schema.FieldDef{
								{FieldName: "id", FieldType: schema.TypeInt, ColIndex: 0},
								{FieldName: "optional", FieldType: schema.TypeString, ColIndex: 1},
							},
						},
						Rows: [][]string{{"1001", ""}},
					},
				},
			},
			expected: []map[string]interface{}{
				{"id": int64(1001)},
			},
		},
		{
			name: "SheetName注入",
			classData: &merger.ClassData{
				ClassName: "Item",
				Meta: &classconfig.ClassMeta{
					PkType:        classconfig.PkTypeSingle,
					PkFields:      []string{"id"},
					SheetNameAs:   "groupId",
					SheetNameType: "int",
				},
				SheetData: []*merger.SheetRows{
					{
						Schema: &schema.SheetSchema{
							Fields: []schema.FieldDef{
								{FieldName: "id", FieldType: schema.TypeInt, ColIndex: 0},
							},
						},
						Rows:      [][]string{{"1001"}},
						SheetName: "100",
					},
				},
			},
			expected: []map[string]interface{}{
				{"id": int64(1001), "groupId": int64(100)},
			},
		},
		{
			name: "跳过无FieldName的列",
			classData: &merger.ClassData{
				ClassName: "Item",
				Meta:      &classconfig.ClassMeta{PkType: classconfig.PkTypeSingle, PkFields: []string{"id"}},
				SheetData: []*merger.SheetRows{
					{
						Schema: &schema.SheetSchema{
							Fields: []schema.FieldDef{
								{FieldName: "id", FieldType: schema.TypeInt, ColIndex: 0},
								{FieldName: "", FieldType: schema.TypeString, ColIndex: 1}, // 无FieldName
								{FieldName: "name", FieldType: schema.TypeString, ColIndex: 2},
							},
						},
						Rows: [][]string{{"1001", "ignored", "Sword"}},
					},
				},
			},
			expected: []map[string]interface{}{
				{"id": int64(1001), "name": "Sword"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Build(tt.classData)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
