package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranspose(t *testing.T) {
	tests := []struct {
		name     string
		cols     [][]string
		expected [][]string
	}{
		{
			name:     "空输入",
			cols:     [][]string{},
			expected: [][]string{},
		},
		{
			name: "单列单行",
			cols: [][]string{
				{"a"},
			},
			expected: [][]string{
				{"a"},
			},
		},
		{
			name: "单列多行",
			cols: [][]string{
				{"a", "b", "c"},
			},
			expected: [][]string{
				{"a"},
				{"b"},
				{"c"},
			},
		},
		{
			name: "多列单行",
			cols: [][]string{
				{"a"},
				{"b"},
				{"c"},
			},
			expected: [][]string{
				{"a", "b", "c"},
			},
		},
		{
			name: "方阵",
			cols: [][]string{
				{"a1", "a2", "a3"},
				{"b1", "b2", "b3"},
				{"c1", "c2", "c3"},
			},
			expected: [][]string{
				{"a1", "b1", "c1"},
				{"a2", "b2", "c2"},
				{"a3", "b3", "c3"},
			},
		},
		{
			name: "不规则矩阵-列长度不同",
			cols: [][]string{
				{"a1", "a2"},
				{"b1", "b2", "b3"},
				{"c1"},
			},
			expected: [][]string{
				{"a1", "b1", "c1"},
				{"a2", "b2", ""},
				{"", "b3", ""},
			},
		},
		{
			name: "空列",
			cols: [][]string{
				{"a", "b"},
				{},
				{"c", "d"},
			},
			expected: [][]string{
				{"a", "", "c"},
				{"b", "", "d"},
			},
		},
		{
			name:     "全空列",
			cols:     [][]string{{}, {}},
			expected: [][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Transpose(tt.cols)
			assert.Equal(t, tt.expected, result)
		})
	}
}
