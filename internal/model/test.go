package model

import "encoding/json"

type TestDTO struct {
	Stdin  string `json:"stdin"`
	Stdout string `json:"stdout"`
}

type Test struct {
	ID     int    `json:"id"`
	Stdin  string `json:"stdin"`
	Stdout string `json:"stdout"`
}

type TestResult struct {
	TaskID     string `json:"task_id"`
	TestID     int    `json:"test_id"`
	Successful bool   `json:"successful"`
}

func ParseTestsJSON(data []byte) ([]TestDTO, error) {
	var tests []TestDTO
	err := json.Unmarshal(data, &tests)
	return tests, err
}
