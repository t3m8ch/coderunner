package model

import "encoding/json"

type Test struct {
	Stdin  string `json:"stdin"`
	Stdout string `json:"stdout"`
}

func ParseTestsJSON(data []byte) ([]Test, error) {
	var tests []Test
	err := json.Unmarshal(data, &tests)
	return tests, err
}
