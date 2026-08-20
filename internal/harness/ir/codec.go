package ir

import "encoding/json"

func Marshal(def Definition) ([]byte, error) {
	return json.Marshal(def)
}

func MarshalIndent(def Definition) ([]byte, error) {
	return json.MarshalIndent(def, "", "  ")
}

func Unmarshal(data []byte) (Definition, error) {
	var def Definition
	err := json.Unmarshal(data, &def)
	return def, err
}
