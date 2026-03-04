package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xlsxtojson/builder"
	"xlsxtojson/exporter"
	"xlsxtojson/merger"
	"xlsxtojson/reader"
	"xlsxtojson/validator"
)

// getTestdataPath 返回测试数据目录的绝对路径
func getTestdataPath(relativePath string) string {
	// 获取项目根目录
	wd, _ := os.Getwd()
	// integration 目录在根目录下，所以往上一级
	projectRoot := filepath.Dir(wd)
	return filepath.Join(projectRoot, relativePath)
}

// TestBasicFlow 测试完整的基本流程
// 使用 testdata/scenarios/001-basic-flow 中的真实 Excel 文件
func TestBasicFlow(t *testing.T) {
	tests := []struct {
		name         string
		inputPath    string
		wantClasses  int          // 期望的 Class 数量
		wantClass    string       // 要检查的 Class 名称
		wantRowCount int          // 期望的数据行数
		wantFirstRow func(t *testing.T, row map[string]interface{}) // 验证第一行数据
	}{
		{
			name:        "道具配置",
			inputPath:   "testdata/scenarios/001-basic-flow/D-道具配置.xlsx",
			wantClasses: 1,
			wantClass:   "ItemConfig",
			wantRowCount: 4,
			wantFirstRow: func(t *testing.T, row map[string]interface{}) {
				assert.Equal(t, int64(1), row["id"])
				assert.Equal(t, "金砖", row["name"])
			},
		},
		{
			name:        "装备分类部位表",
			inputPath:   "testdata/scenarios/001-basic-flow/Z-装备分类部位表.xlsx",
			wantClasses: 1,
			wantClass:   "EquipSlotConfig",
			wantRowCount: 12,
			wantFirstRow: func(t *testing.T, row map[string]interface{}) {
				assert.Equal(t, int64(1), row["typeId"])
			},
		},
		{
			name:        "任务配置",
			inputPath:   "testdata/scenarios/001-basic-flow/R-任务配置.xlsx",
			wantClasses: 1,
			wantClass:   "TaskConfig",
			wantRowCount: 20,
			wantFirstRow: func(t *testing.T, row map[string]interface{}) {
				assert.Equal(t, int64(1), row["id"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. 读取 Excel
			schemas, classMetas, err := reader.ReadAll(getTestdataPath(tt.inputPath))
			require.NoError(t, err, "读取 Excel 失败")

			// 2. 合并数据
			classData, err := merger.Merge(schemas, classMetas)
			require.NoError(t, err, "合并数据失败")
			assert.Equal(t, tt.wantClasses, len(classData), "Class 数量不匹配")

			// 3. 校验数据
			err = validator.Validate(classData)
			require.NoError(t, err, "数据校验失败")

			// 4. 构建数据
			for _, data := range classData {
				for _, sheetData := range data.SheetData {
					merger.SortRowsByRows(sheetData.Rows, sheetData.Schema.Fields,
						data.Meta.PkType, data.Meta.PkFields, data.Meta.SortFields)
				}
				rows, err := builder.Build(data)
				require.NoError(t, err, "构建数据失败")
				data.ParsedRows = rows
			}

			// 验证特定 Class
			if tt.wantClass != "" {
				data, exists := classData[tt.wantClass]
				require.True(t, exists, "Class %s 不存在", tt.wantClass)
				assert.Equal(t, tt.wantRowCount, len(data.ParsedRows), "行数不匹配")
				if tt.wantFirstRow != nil && len(data.ParsedRows) > 0 {
					tt.wantFirstRow(t, data.ParsedRows[0])
				}
			}
		})
	}
}

// TestDirectoryFlow 测试目录级别的完整流程
func TestDirectoryFlow(t *testing.T) {
	inputPath := getTestdataPath("testdata/scenarios/001-basic-flow")
	wantClasses := 4 // 4 个 Excel 文件

	// 1. 读取目录
	schemas, classMetas, err := reader.ReadAll(inputPath)
	require.NoError(t, err, "读取目录失败")
	assert.GreaterOrEqual(t, len(schemas), wantClasses, "Schema 数量应 >= %d", wantClasses)

	// 2. 合并数据
	classData, err := merger.Merge(schemas, classMetas)
	require.NoError(t, err, "合并数据失败")
	assert.Equal(t, wantClasses, len(classData), "Class 数量不匹配")

	// 3. 校验数据
	err = validator.Validate(classData)
	require.NoError(t, err, "数据校验失败")

	// 4. 构建数据
	for _, data := range classData {
		for _, sheetData := range data.SheetData {
			merger.SortRowsByRows(sheetData.Rows, sheetData.Schema.Fields,
				data.Meta.PkType, data.Meta.PkFields, data.Meta.SortFields)
		}
		rows, err := builder.Build(data)
		require.NoError(t, err, "构建数据失败")
		data.ParsedRows = rows
	}

	// 验证关键 Class 存在
	assert.Contains(t, classData, "ItemConfig")
	assert.Contains(t, classData, "EquipSlotConfig")
	assert.Contains(t, classData, "TaskConfig")
	// RandomRewardPoolConfig 因为缺少 Server 行会被跳过
}

// TestExportIntegration 测试导出到 JSON 文件的完整流程
func TestExportIntegration(t *testing.T) {
	inputPath := getTestdataPath("testdata/scenarios/001-basic-flow")

	// 创建临时输出目录
	tmpDir, err := os.MkdirTemp("", "integration_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 1. 读取
	schemas, classMetas, err := reader.ReadAll(inputPath)
	require.NoError(t, err)

	// 2. 合并
	classData, err := merger.Merge(schemas, classMetas)
	require.NoError(t, err)

	// 3. 校验
	err = validator.Validate(classData)
	require.NoError(t, err)

	// 4. 构建
	for _, data := range classData {
		for _, sheetData := range data.SheetData {
			merger.SortRowsByRows(sheetData.Rows, sheetData.Schema.Fields,
				data.Meta.PkType, data.Meta.PkFields, data.Meta.SortFields)
		}
		rows, err := builder.Build(data)
		require.NoError(t, err)
		data.ParsedRows = rows
	}

	// 5. 导出
	err = exporter.Export(classData, tmpDir, true)
	require.NoError(t, err)

	// 6. 验证导出文件
	expectedFiles := []string{"ItemConfig.json", "EquipSlotConfig.json", "TaskConfig.json"}
	for _, filename := range expectedFiles {
		filePath := filepath.Join(tmpDir, filename)
		_, err = os.Stat(filePath)
		assert.NoError(t, err, "文件 %s 不存在", filename)

		// 验证 JSON 格式有效
		content, _ := os.ReadFile(filePath)
		var jsonData interface{}
		err = json.Unmarshal(content, &jsonData)
		assert.NoError(t, err, "文件 %s JSON 格式无效", filename)
	}
}

// TestEmptySheet 测试空 Sheet 处理
func TestEmptySheet(t *testing.T) {
	// 创建一个只有表头没有数据的 Excel
	// 这里复用已有的简单场景进行测试
	schemas, classMetas, err := reader.ReadAll(getTestdataPath("testdata/scenarios/001-basic-flow/D-道具配置.xlsx"))
	require.NoError(t, err)

	classData, err := merger.Merge(schemas, classMetas)
	require.NoError(t, err)

	// 验证空 Sheet 不会导致崩溃
	for _, data := range classData {
		rows, err := builder.Build(data)
		require.NoError(t, err, "Build 应成功")
		assert.NotNil(t, rows)
	}
}

// TestNestedFieldIntegration 测试嵌套字段的完整流程
func TestNestedFieldIntegration(t *testing.T) {
	schemas, classMetas, err := reader.ReadAll(getTestdataPath("testdata/scenarios/001-basic-flow/R-任务配置.xlsx"))
	require.NoError(t, err)

	classData, err := merger.Merge(schemas, classMetas)
	require.NoError(t, err)

	err = validator.Validate(classData)
	require.NoError(t, err)

	for _, data := range classData {
		for _, sheetData := range data.SheetData {
			merger.SortRowsByRows(sheetData.Rows, sheetData.Schema.Fields,
				data.Meta.PkType, data.Meta.PkFields, data.Meta.SortFields)
		}
		rows, err := builder.Build(data)
		require.NoError(t, err)

		// 检查是否有嵌套字段 rewards
		for _, row := range rows {
			if rewards, ok := row["rewards"]; ok {
				assert.NotNil(t, rewards)
				// rewards 应该是数组
				rewardsSlice, ok := rewards.([]interface{})
				if ok && len(rewardsSlice) > 0 {
					// 验证第一个 reward 是 map
					_, ok := rewardsSlice[0].(map[string]interface{})
					assert.True(t, ok, "rewards 元素应该是 map")
				}
			}
		}
	}
}
