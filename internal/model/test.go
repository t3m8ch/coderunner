package model

import "encoding/json"

type Test struct {
	Stdin  string `json:"stdin"`
	Stdout string `json:"stdout"`
}

type TestResult struct {
	TaskID     string `json:"task_id"`
	TestID     int    `json:"test_id"`
	Successful bool   `json:"successful"`
}

func ParseTestsJSON(data []byte) ([]Test, error) {
	var tests []Test
	err := json.Unmarshal(data, &tests)
	return tests, err
}
