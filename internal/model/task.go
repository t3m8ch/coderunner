package model

type StartTaskCommand struct {
	ID            string `json:"id"`
	CodeLocation  string `json:"codeLocation"`
	TestsLocation string `json:"testsLocation"`
	Compiler      string `json:"compiler"`
}

type TaskState struct {
	ID            string `json:"id"`
	CodeLocation  string `json:"codeLocation"`
	TestsLocation string `json:"testsLocation"`
	Compiler      string `json:"compiler"`
	State         string `json:"state"`
}

const PENDING_TASK_STATE = "pending"
