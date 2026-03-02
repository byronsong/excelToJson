package builder

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// PathSegment 路径段
type PathSegment struct {
	Name     string // 字段名
	ArrayIdx int    // 数组索引，-1 表示不是数组
	MapKey   int    // Map 键，-1 表示不是 Map
	IsArray  bool
	IsMap    bool
}

// ParsePath 解析列名路径
// 例如: rewards[0].id -> [rewards[0], id] -> [PathSegment{Name:"rewards", ArrayIdx:0}, PathSegment{Name:"id"}]
func ParsePath(fieldName string) ([]PathSegment, error) {
	// 先按 . 分割
	parts := strings.Split(fieldName, ".")
	segments := make([]PathSegment, 0, len(parts))

	for _, part := range parts {
		segment, err := parseSegment(part)
		if err != nil {
			return nil, err
		}
		segments = append(segments, segment)
	}

	return segments, nil
}

// parseSegment 解析单个路径段
func parseSegment(part string) (PathSegment, error) {
	segment := PathSegment{
		ArrayIdx: -1,
		MapKey:   -1,
	}

	// 检查数组索引 [N]
	arrayRegex := regexp.MustCompile(`(\w+)\[(\d+)\]`)
	if match := arrayRegex.FindStringSubmatch(part); match != nil {
		segment.Name = match[1]
		segment.IsArray = true
		idx, err := strconv.Atoi(match[2])
		if err != nil {
			return segment, fmt.Errorf("数组索引解析失败: %s", part)
		}
		segment.ArrayIdx = idx
		return segment, nil
	}

	// 检查 Map 键 {K}
	mapRegex := regexp.MustCompile(`(\w+)\{(\d+)\}`)
	if match := mapRegex.FindStringSubmatch(part); match != nil {
		segment.Name = match[1]
		segment.IsMap = true
		key, err := strconv.Atoi(match[2])
		if err != nil {
			return segment, fmt.Errorf("Map键解析失败: %s", part)
		}
		segment.MapKey = key
		return segment, nil
	}

	// 普通字段
	segment.Name = part
	return segment, nil
}

// SetValueByPath 按路径设置值
// 例如: data["rewards"][0]["id"] = 2001
func SetValueByPath(data map[string]interface{}, segments []PathSegment, value interface{}) error {
	if len(segments) == 0 {
		return fmt.Errorf("路径不能为空")
	}

	// 逐层访问/创建
	current := data

	for i := 0; i < len(segments)-1; i++ {
		seg := segments[i]
		nextSeg := segments[i+1]

		key := seg.Name

		// 如果当前节点不存在，创建它
		if _, exists := current[key]; !exists {
			// 根据下一段的类型决定创建什么类型
			if nextSeg.IsArray {
				current[key] = make([]interface{}, 0)
			} else if nextSeg.IsMap {
				current[key] = make(map[int]interface{})
			} else {
				current[key] = make(map[string]interface{})
			}
		}

		// 访问下一层
		if seg.IsArray {
			var arr []interface{}
			var ok bool
			arr, ok = current[key].([]interface{})
			if !ok {
				// 如果不存在，创建新的数组
				arr = make([]interface{}, 0)
				current[key] = arr
			}
			// 扩展数组 if needed
			extended := false
			for len(arr) <= seg.ArrayIdx {
				arr = append(arr, nil)
				extended = true
			}
			// 如果数组被扩展了，需要更新 map 中的引用
			if extended {
				current[key] = arr
			}
			if arr[seg.ArrayIdx] == nil {
				// 创建下一层
				if nextSeg.IsArray {
					arr[seg.ArrayIdx] = make([]interface{}, 0)
				} else if nextSeg.IsMap {
					arr[seg.ArrayIdx] = make(map[int]interface{})
				} else {
					arr[seg.ArrayIdx] = make(map[string]interface{})
				}
			}
			current = arr[seg.ArrayIdx].(map[string]interface{})
		} else if seg.IsMap {
			m := current[key].(map[int]interface{})
			if _, exists := m[seg.MapKey]; !exists {
				if nextSeg.IsArray {
					m[seg.MapKey] = make([]interface{}, 0)
				} else if nextSeg.IsMap {
					m[seg.MapKey] = make(map[int]interface{})
				} else {
					m[seg.MapKey] = make(map[string]interface{})
				}
			}
			current = m[seg.MapKey].(map[string]interface{})
		} else {
			// 普通对象
			var ok bool
			current, ok = current[key].(map[string]interface{})
			if !ok {
				return fmt.Errorf("路径冲突: %s 不是对象类型", key)
			}
		}
	}

	// 设置最终值
	lastSeg := segments[len(segments)-1]
	key := lastSeg.Name

	if lastSeg.IsArray {
		arr, ok := current[key].([]interface{})
		if !ok {
			// 如果不存在或类型不对，创建新的数组
			arr = make([]interface{}, 0)
			current[key] = arr
		}
		// 扩展数组 if needed
		extended := false
		for len(arr) <= lastSeg.ArrayIdx {
			arr = append(arr, nil)
			extended = true
		}
		// 如果数组被扩展了，需要更新 map 中的引用
		if extended {
			current[key] = arr
		}
		arr[lastSeg.ArrayIdx] = value
	} else if lastSeg.IsMap {
		m := current[key].(map[int]interface{})
		m[lastSeg.MapKey] = value
		current[key] = m
	} else {
		// 检查是否冲突
		if _, exists := current[key]; exists {
			return fmt.Errorf("嵌套路径冲突: %s 已被赋值", key)
		}
		current[key] = value
	}

	return nil
}

// GetValueByPath 按路径获取值
func GetValueByPath(data map[string]interface{}, segments []PathSegment) (interface{}, error) {
	if len(segments) == 0 {
		return nil, fmt.Errorf("路径不能为空")
	}

	current := data

	for _, seg := range segments {
		key := seg.Name

		if val, exists := current[key]; !exists {
			return nil, nil
		} else if seg.IsArray {
			arr := val.([]interface{})
			if seg.ArrayIdx >= len(arr) {
				return nil, nil
			}
			current = arr[seg.ArrayIdx].(map[string]interface{})
		} else if seg.IsMap {
			m := val.(map[int]interface{})
			if v, ok := m[seg.MapKey]; !ok {
				return nil, nil
			} else {
				current = v.(map[string]interface{})
			}
		} else {
			var ok bool
			current, ok = val.(map[string]interface{})
			if !ok {
				return nil, nil
			}
		}
	}

	return current, nil
}
