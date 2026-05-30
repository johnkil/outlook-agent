package transport

import (
	"fmt"
	"strconv"
	"strings"
)

const DefaultPageSize = 150
const MaxPageSize = 250

type PageSizeResult struct {
	Value   int
	Clamped bool
}

func ClampPageSize(value any, defaultSize int, maxSize int) (PageSizeResult, error) {
	if defaultSize <= 0 {
		return PageSizeResult{}, fmt.Errorf("default page size must be positive")
	}
	if maxSize < defaultSize {
		return PageSizeResult{}, fmt.Errorf("max page size must be greater than or equal to default page size")
	}

	size, ok, err := pageSizeInt(value)
	if err != nil {
		return PageSizeResult{}, err
	}
	if !ok || size <= 0 {
		return PageSizeResult{Value: defaultSize}, nil
	}
	if size > maxSize {
		return PageSizeResult{Value: maxSize, Clamped: true}, nil
	}
	return PageSizeResult{Value: size}, nil
}

func pageSizeInt(value any) (int, bool, error) {
	switch typed := value.(type) {
	case nil:
		return 0, false, nil
	case int:
		return typed, true, nil
	case int8:
		return int(typed), true, nil
	case int16:
		return int(typed), true, nil
	case int32:
		return int(typed), true, nil
	case int64:
		if typed > int64(maxInt()) {
			return maxInt(), true, nil
		}
		return int(typed), true, nil
	case uint:
		if typed > uint(maxInt()) {
			return maxInt(), true, nil
		}
		return int(typed), true, nil
	case uint8:
		return int(typed), true, nil
	case uint16:
		return int(typed), true, nil
	case uint32:
		if uint64(typed) > uint64(maxInt()) {
			return maxInt(), true, nil
		}
		return int(typed), true, nil
	case uint64:
		if typed > uint64(maxInt()) {
			return maxInt(), true, nil
		}
		return int(typed), true, nil
	case float32:
		return pageSizeFloat(float64(typed))
	case float64:
		return pageSizeFloat(typed)
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0, false, nil
		}
		parsed, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return 0, false, fmt.Errorf("invalid page size %q", text)
		}
		if parsed > int64(maxInt()) {
			return maxInt(), true, nil
		}
		return int(parsed), true, nil
	default:
		return 0, false, fmt.Errorf("invalid page size type %T", value)
	}
}

func pageSizeFloat(value float64) (int, bool, error) {
	if value != value {
		return 0, false, fmt.Errorf("page size must be a number")
	}
	if value > float64(maxInt()) {
		return maxInt(), true, nil
	}
	if value < -float64(maxInt()) {
		return -maxInt(), true, nil
	}
	if value != float64(int64(value)) {
		return 0, false, fmt.Errorf("page size must be an integer")
	}
	return int(value), true, nil
}

func maxInt() int {
	return int(^uint(0) >> 1)
}
