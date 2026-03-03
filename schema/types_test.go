package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFieldType(t *testing.T) {
	tests := []struct {
		name     string
		typeStr  string
		expected FieldType
	}{
		// 基础类型
		{"int", "int", TypeInt},
		{"int64", "int64", TypeInt},
		{"float", "float", TypeFloat},
		{"float64", "float64", TypeFloat},
		{"string", "string", TypeString},
		{"bool", "bool", TypeBool},

		// 数组类型
		{"int slice", "[]int", TypeIntSlice},
		{"float slice", "[]float", TypeFloatSlice},
		{"string slice", "[]string", TypeStringSlice},

		// Map类型
		{"int-int map", "map<int,int>", TypeIntMap},
		{"int-float map", "map<int,float>", TypeFloatMap},
		{"string-int map", "map<string,int>", TypeStringMap},
		{"int-string map", "map<int,string>", TypeIntStringMap},

		// 结构体类型
		{"struct", "struct", TypeStruct},
		{"struct slice", "[]struct", TypeStructSlice},
		{"struct map", "map<int,struct>", TypeStructMap},

		// 特殊类型
		{"ignore", "ignore", TypeIgnore},
		{"empty", "", TypeUnknown},
		{"unknown", "invalid_type", TypeUnknown},
		{"chinese type", "整数", TypeUnknown},
		{"space in type", " int ", TypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseFieldType(tt.typeStr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFieldTypeString(t *testing.T) {
	tests := []struct {
		fieldType FieldType
		expected  string
	}{
		{TypeInt, "int"},
		{TypeFloat, "float"},
		{TypeString, "string"},
		{TypeBool, "bool"},
		{TypeIntSlice, "[]int"},
		{TypeFloatSlice, "[]float"},
		{TypeStringSlice, "[]string"},
		{TypeIntMap, "map<int,int>"},
		{TypeFloatMap, "map<int,float>"},
		{TypeStringMap, "map<string,int>"},
		{TypeIntStringMap, "map<int,string>"},
		{TypeStruct, "struct"},
		{TypeStructSlice, "[]struct"},
		{TypeStructMap, "map<int,struct>"},
		{TypeIgnore, "ignore"},
		{TypeUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.fieldType.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}
