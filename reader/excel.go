package reader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xlsxtojson/schema"

	"github.com/xuri/excelize/v2"
)

// ReadExcel 读取 Excel 文件并解析所有 Sheet
func ReadExcel(filePath string) ([]*schema.SheetSchema, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	fileName := filepath.Base(filePath)

	var schemas []*schema.SheetSchema

	for _, sheetName := range sheets {
		// 读取整个 Sheet 的数据（包括空单元格）
		rows, err := f.GetCols(sheetName)
		if err != nil {
			return nil, fmt.Errorf("%s / %s: 读取Sheet失败: %w", fileName, sheetName, err)
		}

		// GetCols 返回的是列，需要转换为行
		rows = transpose(rows)

		// 跳过空 Sheet
		if len(rows) == 0 {
			continue
		}

		// 解析表头
		schema, err := schema.ParseHeader(rows, fileName, sheetName)
		if err != nil {
			// 如果是 A1 为空或未找到 FieldName 行的警告，打印警告并跳过
			if strings.Contains(err.Error(), "A1 为空") || strings.Contains(err.Error(), "未找到 FieldName 行") {
				fmt.Printf("[WARN] %s\n", err.Error())
				continue
			}
			return nil, err
		}

		schemas = append(schemas, schema)
	}

	return schemas, nil
}

// ScanDirectory 扫描目录下所有 xlsx 文件
func ScanDirectory(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 排除临时文件（Excel 临时文件以 ~$ 开头）
		if strings.HasPrefix(name, "~$") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(name), ".xlsx") {
			files = append(files, filepath.Join(dirPath, name))
		}
	}

	return files, nil
}

// ReadAll 读取输入路径下的所有 Excel 文件
func ReadAll(inputPath string) ([]*schema.SheetSchema, error) {
	var allSchemas []*schema.SheetSchema

	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, fmt.Errorf("输入路径不存在: %w", err)
	}

	if info.IsDir() {
		// 是目录，扫描所有 xlsx 文件
		files, err := ScanDirectory(inputPath)
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("目录中没有找到 .xlsx 文件: %s", inputPath)
		}

		for _, file := range files {
			schemas, err := ReadExcel(file)
			if err != nil {
				return nil, err
			}
			allSchemas = append(allSchemas, schemas...)
		}
	} else {
		// 是单个文件
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".xlsx") {
			return nil, fmt.Errorf("输入文件不是 .xlsx 格式: %s", info.Name())
		}
		schemas, err := ReadExcel(inputPath)
		if err != nil {
			return nil, err
		}
		allSchemas = append(allSchemas, schemas...)
	}

	return allSchemas, nil
}

// transpose 将列数据转换为行数据
// GetCols 返回 [][]string，每一行是一列的数据
// 需要转换为传统的行数据格式
func transpose(cols [][]string) [][]string {
	if len(cols) == 0 {
		return [][]string{}
	}

	// 找出最长的列
	maxRows := 0
	for _, col := range cols {
		if len(col) > maxRows {
			maxRows = len(col)
		}
	}

	// 创建行数据，初始化为空字符串
	rows := make([][]string, maxRows)
	for i := range rows {
		rows[i] = make([]string, len(cols))
		for j := range rows[i] {
			rows[i][j] = ""
		}
	}

	// 填充数据
	for colIdx, col := range cols {
		for rowIdx, val := range col {
			rows[rowIdx][colIdx] = val
		}
	}

	return rows
}
