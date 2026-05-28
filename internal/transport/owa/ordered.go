package owa

import (
	"bytes"
	"encoding/json"
	"sort"
)

type orderedObject []orderedField

type orderedField struct {
	name  string
	value any
}

func object(fields ...orderedField) orderedObject {
	return orderedObject(fields)
}

func field(name string, value any) orderedField {
	return orderedField{name: name, value: value}
}

func (value orderedObject) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte('{')
	for index, field := range value {
		if index > 0 {
			buffer.WriteByte(',')
		}
		name, err := json.Marshal(field.name)
		if err != nil {
			return nil, err
		}
		payload, err := json.Marshal(field.value)
		if err != nil {
			return nil, err
		}
		buffer.Write(name)
		buffer.WriteByte(':')
		buffer.Write(payload)
	}
	buffer.WriteByte('}')
	return buffer.Bytes(), nil
}

func orderTypeFieldsFirst(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		fields := make([]orderedField, 0, len(typed))
		if typeValue, ok := typed["__type"]; ok {
			fields = append(fields, field("__type", orderTypeFieldsFirst(typeValue)))
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			if key != "__type" {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			fields = append(fields, field(key, orderTypeFieldsFirst(typed[key])))
		}
		return object(fields...)
	case []any:
		values := make([]any, len(typed))
		for index, item := range typed {
			values[index] = orderTypeFieldsFirst(item)
		}
		return values
	default:
		return value
	}
}
