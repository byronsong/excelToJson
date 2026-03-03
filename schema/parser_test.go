package schema

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHeader(t *testing.T) {
	tests := []struct {
		name        string
		rows        [][]string
		wantErr     bool
		errContains string
		validate    func(t *testing.T, s *SheetSchema)
	}{
		{
			name: "成功解析-标准表头（4行）",
			rows: [][]string{
				{"ItemConfig", "道具ID"},
				{"Type", "int"},
				{"Client", "id"},
				{"Server", "id"},
			},
			wantErr: false,
			validate: func(t *testing.T, s *SheetSchema) {
				assert.Equal(t, "ItemConfig", s.ClassName)
				assert.Equal(t, 1, len(s.Fields))
				assert.Equal(t, "id", s.Fields[0].FieldName)
				assert.Equal(t, TypeInt, s.Fields[0].FieldType)
			},
		},
		{
			name: "A1为空",
			rows: [][]string{
				{"", "道具ID"},
				{"Type", "int"},
				{"Client", "id"},
				{"Server", "id"},
			},
			wantErr:     true,
			errContains: "A1 为空",
		},
		{
			name: "缺少Server标识行",
			rows: [][]string{
				{"ItemConfig", "道具ID"},
				{"Type", "int"},
				{"Client", "id"},
				{"", "id"},
			},
			wantErr:     true,
			errContains: "未找到 FieldName 行",
		},
		{
			name: "嵌套字段识别",
			rows: [][]string{
				{"ItemConfig", "奖励"},
				{"Type", "[]struct"},
				{"Client", "rewards"},
				{"Server", "rewards[0].id"},
			},
			wantErr: false,
			validate: func(t *testing.T, s *SheetSchema) {
				assert.Equal(t, "rewards[0].id", s.Fields[0].FieldName)
			},
		},
		{
			name: "ignore类型列",
			rows: [][]string{
				{"ItemConfig", "道具ID", "备注"},
				{"Type", "int", "ignore"},
				{"Client", "id", "remark"},
				{"Server", "id", "#remark"},
			},
			wantErr: false,
			validate: func(t *testing.T, s *SheetSchema) {
				assert.Equal(t, 2, len(s.Fields))
				// ignore 类型的字段应该被标记为 Ignored
				for _, f := range s.Fields {
					if f.TypeStr == "ignore" || f.FieldName == "#remark" {
						assert.True(t, f.Ignored)
					}
				}
			},
		},
		{
			name: "空Sheet（无数据行）",
			rows: [][]string{
				{"ItemConfig", "道具ID"},
				{"Type", "int"},
				{"Client", "id"},
				{"Server", "id"},
			},
			wantErr: false,
			validate: func(t *testing.T, s *SheetSchema) {
				assert.Equal(t, 0, len(s.DataRows))
			},
		},
		{
			name: "多列解析",
			rows: [][]string{
				{"ItemConfig", "ID", "名称", "价格"},
				{"Type", "int", "string", "int"},
				{"Client", "id", "name", "price"},
				{"Server", "id", "name", "price"},
			},
			wantErr: false,
			validate: func(t *testing.T, s *SheetSchema) {
				assert.Equal(t, 3, len(s.Fields))
				assert.Equal(t, "id", s.Fields[0].FieldName)
				assert.Equal(t, "name", s.Fields[1].FieldName)
				assert.Equal(t, "price", s.Fields[2].FieldName)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := ParseHeader(tt.rows, "test.xlsx", "Sheet1")
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, schema)
			}
		})
	}
}

func TestParseHeader_Boundary(t *testing.T) {
	tests := []struct {
		name string
		rows [][]string
	}{
		{
			name: "100列宽表",
			rows: func() [][]string {
				rows := make([][]string, 4)
				rows[0] = append([]string{"Config"}, make([]string, 100)...)
				rows[1] = append([]string{"Type"}, make([]string, 100)...)
				rows[2] = append([]string{"Client"}, make([]string, 100)...)
				rows[3] = append([]string{"Server"}, make([]string, 100)...)
				for i := 0; i < 100; i++ {
					rows[1][i+1] = "int"
					rows[3][i+1] = fmt.Sprintf("field%d", i)
				}
				return rows
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseHeader(tt.rows, "test.xlsx", "Sheet1")
			assert.NoError(t, err)
		})
	}
}
