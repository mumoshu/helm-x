package x

import (
	"bytes"
	"gopkg.in/yaml.v3"
)

func YamlMarshal(v interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	marshaller := yaml.NewEncoder(buf)
	marshaller.SetIndent(2)

	if err := marshaller.Encode(v); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
