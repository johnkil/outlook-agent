package owa

import (
	"bytes"
	"encoding/json"
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
