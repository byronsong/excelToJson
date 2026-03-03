package exporter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"xlsxtojson/classconfig"
	"xlsxtojson/merger"
)

func TestExport(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]*merger.ClassData
		pretty    bool
		wantFiles map[string]string // filename -> expected content substring
		wantErr   bool
	}{
		{
			name: "基本导出",
			data: map[string]*merger.ClassData{
				"Item": {
					ClassName: "Item",
					Meta:      &classconfig.ClassMeta{},
					ParsedRows: []map[string]interface{}{
						{"id": int64(1), "name": "Sword"},
						{"id": int64(2), "name": "Shield"},
					},
				},
			},
			pretty: false,
			wantFiles: map[string]string{
				"Item.json": `"id":1`,
			},
			wantErr: false,
		},
		{
			name: "格式化导出",
			data: map[string]*merger.ClassData{
				"Item": {
					ClassName: "Item",
					Meta:      &classconfig.ClassMeta{},
					ParsedRows: []map[string]interface{}{
						{"id": int64(1), "name": "Sword"},
					},
				},
			},
			pretty: true,
			wantFiles: map[string]string{
				"Item.json": "  \"id\": 1",
			},
			wantErr: false,
		},
		{
			name: "多Class导出",
			data: map[string]*merger.ClassData{
				"Item": {
					ClassName:  "Item",
					Meta:       &classconfig.ClassMeta{},
					ParsedRows: []map[string]interface{}{{"id": int64(1)}},
				},
				"Equip": {
					ClassName:  "Equip",
					Meta:       &classconfig.ClassMeta{},
					ParsedRows: []map[string]interface{}{{"id": int64(100)}},
				},
			},
			pretty: false,
			wantFiles: map[string]string{
				"Item.json":  `"id":1`,
				"Equip.json": `"id":100`,
			},
			wantErr: false,
		},
		{
			name: "空数据",
			data: map[string]*merger.ClassData{
				"Empty": {
					ClassName:  "Empty",
					Meta:       &classconfig.ClassMeta{},
					ParsedRows: []map[string]interface{}{},
				},
			},
			pretty: false,
			wantFiles: map[string]string{
				"Empty.json": "[]",
			},
			wantErr: false,
		},
		{
			name: "嵌套结构",
			data: map[string]*merger.ClassData{
				"Item": {
					ClassName: "Item",
					Meta:      &classconfig.ClassMeta{},
					ParsedRows: []map[string]interface{}{
						{
							"id":      int64(1),
							"rewards": []interface{}{map[string]interface{}{"id": int64(100), "count": int64(5)}},
						},
					},
				},
			},
			pretty: true,
			wantFiles: map[string]string{
				"Item.json": "rewards",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建临时目录
			tmpDir, err := os.MkdirTemp("", "exporter_test")
			assert.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			// 执行导出
			err = Export(tt.data, tmpDir, tt.pretty)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// 验证文件内容
			for filename, expectedSubstr := range tt.wantFiles {
				filePath := filepath.Join(tmpDir, filename)
				content, err := os.ReadFile(filePath)
				assert.NoError(t, err, "读取文件 %s 失败", filename)
				assert.Contains(t, string(content), expectedSubstr, "文件 %s 内容不符合预期", filename)
			}
		})
	}
}

func TestExport_CreateDir(t *testing.T) {
	// 测试自动创建目录
	tmpDir, err := os.MkdirTemp("", "exporter_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	nestedDir := filepath.Join(tmpDir, "nested", "dir")

	data := map[string]*merger.ClassData{
		"Item": {
			ClassName:  "Item",
			Meta:       &classconfig.ClassMeta{},
			ParsedRows: []map[string]interface{}{{"id": int64(1)}},
		},
	}

	err = Export(data, nestedDir, false)
	assert.NoError(t, err)

	// 验证文件存在
	filePath := filepath.Join(nestedDir, "Item.json")
	_, err = os.ReadFile(filePath)
	assert.NoError(t, err)
}
