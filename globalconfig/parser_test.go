package globalconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGlobalConfig(t *testing.T) {
	tests := []struct {
		name        string
		rows        [][]string
		wantCount   int
		wantErr     bool
		checkFirst  func(t *testing.T, entries []*GlobalEntry)
	}{
		{
			name: "基本类型测试",
			rows: [][]string{
				{"!GlobalConfig", "唯一标识", "值类型", "内容"},
				{"Type", "string", "string", "string"},
				{"id", "type", "value"}, // FieldName 行
				{"", "battle:maxLevel", "", "100"},
				{"", "battle:version", "", "1.0.0"},
				{"", "battle:enablePvp", "", "true"},
			},
			wantCount: 3,
			wantErr:   false,
			checkFirst: func(t *testing.T, entries []*GlobalEntry) {
				assert.Equal(t, "battle:maxLevel", entries[0].ID)
				assert.Equal(t, int64(100), entries[0].Value)
				assert.Equal(t, "battle:version", entries[1].ID)
				assert.Equal(t, "1.0.0", entries[1].Value)
				assert.Equal(t, "battle:enablePvp", entries[2].ID)
				assert.Equal(t, true, entries[2].Value)
			},
		},
		{
			name: "显式类型声明",
			rows: [][]string{
				{"!GlobalConfig", "唯一标识", "值类型", "内容"},
				{"Type", "string", "string", "string"},
				{"id", "type", "value"}, // FieldName 行
				{"", "items:ids", "[]int", "1001,1002,1003"},
				{"", "map:test", "map<string,int>", "a:1,b:2"},
			},
			wantCount: 2,
			wantErr:   false,
			checkFirst: func(t *testing.T, entries []*GlobalEntry) {
				assert.Equal(t, "items:ids", entries[0].ID)
				slice, ok := entries[0].Value.([]interface{})
				require.True(t, ok, "应该是 slice")
				assert.Equal(t, 3, len(slice))
			},
		},
		{
			name: "空 ID 报错",
			rows: [][]string{
				{"!GlobalConfig", "唯一标识", "值类型", "内容"},
				{"Type", "string", "string", "string"},
				{"id", "type", "value"}, // FieldName 行
				{"", "", "", "value"},
			},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "ID 重复报错",
			rows: [][]string{
				{"!GlobalConfig", "唯一标识", "值类型", "内容"},
				{"Type", "string", "string", "string"},
				{"id", "type", "value"}, // FieldName 行
				{"", "battle:maxLevel", "", "100"},
				{"", "battle:maxLevel", "", "200"},
			},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "float 类型推断",
			rows: [][]string{
				{"!GlobalConfig", "唯一标识", "值类型", "内容"},
				{"Type", "string", "string", "string"},
				{"id", "type", "value"}, // FieldName 行
				{"", "rate:discount", "", "0.85"},
			},
			wantCount: 1,
			wantErr:   false,
			checkFirst: func(t *testing.T, entries []*GlobalEntry) {
				assert.Equal(t, float64(0.85), entries[0].Value)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ParseGlobalConfig(tt.rows, "test.xlsx", "Sheet1")

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, len(data.Entries))

			if tt.checkFirst != nil && len(data.Entries) > 0 {
				tt.checkFirst(t, data.Entries)
			}
		})
	}
}

// TestParseGlobalConfig_WarnPath 测试 WARN 路径
func TestParseGlobalConfig_WarnPath(t *testing.T) {
	tests := []struct {
		name      string
		rows      [][]string
		wantCount int
		wantErr   bool
		checkFirst func(t *testing.T, entries []*GlobalEntry)
	}{
		{
			name: "含逗号的值自动推断为 string",
			rows: [][]string{
				{"!GlobalConfig", "唯一标识", "值类型", "内容"},
				{"Type", "string", "string", "string"},
				{"id", "type", "value"},
				{"", "test:comma", "", "a,b,c"},
			},
			wantCount: 1,
			wantErr:   false,
			checkFirst: func(t *testing.T, entries []*GlobalEntry) {
				assert.Equal(t, "test:comma", entries[0].ID)
				assert.Equal(t, "a,b,c", entries[0].Value) // 推断为 string
			},
		},
		{
			name: "含冒号的值自动推断为 string",
			rows: [][]string{
				{"!GlobalConfig", "唯一标识", "值类型", "内容"},
				{"Type", "string", "string", "string"},
				{"id", "type", "value"},
				{"", "test:colon", "", "k1:v1,k2:v2"},
			},
			wantCount: 1,
			wantErr:   false,
			checkFirst: func(t *testing.T, entries []*GlobalEntry) {
				assert.Equal(t, "test:colon", entries[0].ID)
				assert.Equal(t, "k1:v1,k2:v2", entries[0].Value) // 推断为 string
			},
		},
		{
			name: "显式 float 类型无小数点",
			rows: [][]string{
				{"!GlobalConfig", "唯一标识", "值类型", "内容"},
				{"Type", "string", "string", "string"},
				{"id", "type", "value"},
				{"", "test:float", "float", "5"},
			},
			wantCount: 1,
			wantErr:   false,
			checkFirst: func(t *testing.T, entries []*GlobalEntry) {
				assert.Equal(t, "test:float", entries[0].ID)
				assert.Equal(t, float64(5), entries[0].Value)
			},
		},
		{
			name: "非法类型报错",
			rows: [][]string{
				{"!GlobalConfig", "唯一标识", "值类型", "内容"},
				{"Type", "string", "string", "string"},
				{"id", "type", "value"},
				{"", "test:invalid", "map<bool,int>", "true:1"},
			},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "A 列有内容但 B 列为空",
			rows: [][]string{
				{"!GlobalConfig", "唯一标识", "值类型", "内容"},
				{"Type", "string", "string", "string"},
				{"id", "type", "value"},
				{"some comment", "", "", "value"},
			},
			wantCount: 0,
			wantErr:   false, // 只是 WARN，不报错
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ParseGlobalConfig(tt.rows, "test.xlsx", "Sheet1")

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, len(data.Entries))

			if tt.checkFirst != nil && len(data.Entries) > 0 {
				tt.checkFirst(t, data.Entries)
			}
		})
	}
}

func TestInferAndParse(t *testing.T) {
	tests := []struct {
		name      string
		entry     *GlobalEntry
		wantValue interface{}
	}{
		{
			name: "int 推断",
			entry: &GlobalEntry{
				ID:         "test:1",
				TypeStr:    "",
				RawValue:   "100",
				FileName:   "test.xlsx",
				SheetName:  "Sheet1",
				RowIndex:   4,
			},
			wantValue: int64(100),
		},
		{
			name: "float 推断（含小数点）",
			entry: &GlobalEntry{
				ID:         "test:2",
				TypeStr:    "",
				RawValue:   "3.14",
				FileName:   "test.xlsx",
				SheetName:  "Sheet1",
				RowIndex:   4,
			},
			wantValue: float64(3.14),
		},
		{
			name: "bool 推断",
			entry: &GlobalEntry{
				ID:         "test:3",
				TypeStr:    "",
				RawValue:   "true",
				FileName:   "test.xlsx",
				SheetName:  "Sheet1",
				RowIndex:   4,
			},
			wantValue: true,
		},
		{
			name: "string 推断",
			entry: &GlobalEntry{
				ID:         "test:4",
				TypeStr:    "",
				RawValue:   "hello",
				FileName:   "test.xlsx",
				SheetName:  "Sheet1",
				RowIndex:   4,
			},
			wantValue: "hello",
		},
		{
			name: "显式 int 类型",
			entry: &GlobalEntry{
				ID:         "test:5",
				TypeStr:    "int",
				RawValue:   "200",
				FileName:   "test.xlsx",
				SheetName:  "Sheet1",
				RowIndex:   4,
			},
			wantValue: int64(200),
		},
		{
			name: "显式 string 类型",
			entry: &GlobalEntry{
				ID:         "test:6",
				TypeStr:    "string",
				RawValue:   "hello world",
				FileName:   "test.xlsx",
				SheetName:  "Sheet1",
				RowIndex:   4,
			},
			wantValue: "hello world",
		},
		{
			name: "显式 []int 类型",
			entry: &GlobalEntry{
				ID:         "test:7",
				TypeStr:    "[]int",
				RawValue:   "1,2,3",
				FileName:   "test.xlsx",
				SheetName:  "Sheet1",
				RowIndex:   4,
			},
			wantValue: []interface{}{int64(1), int64(2), int64(3)},
		},
		{
			name: "显式 map<string,int> 类型",
			entry: &GlobalEntry{
				ID:         "test:8",
				TypeStr:    "map<string,int>",
				RawValue:   "a:1,b:2",
				FileName:   "test.xlsx",
				SheetName:  "Sheet1",
				RowIndex:   4,
			},
			wantValue: map[string]interface{}{"a": int64(1), "b": int64(2)},
		},
		{
			name: "显式 map<string,string> 类型",
			entry: &GlobalEntry{
				ID:         "test:9",
				TypeStr:    "map<string,string>",
				RawValue:   "key1:value1,key2:value2",
				FileName:   "test.xlsx",
				SheetName:  "Sheet1",
				RowIndex:   4,
			},
			wantValue: map[string]interface{}{"key1": "value1", "key2": "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := parseValue(tt.entry)
			require.NoError(t, err)
			assert.Equal(t, tt.wantValue, value)
		})
	}
}
