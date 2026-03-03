package util

// Transpose 将列数据转换为行数据
// GetCols 返回 [][]string，每一行是一列的数据
// 需要转换为传统的行数据格式
func Transpose(cols [][]string) [][]string {
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
