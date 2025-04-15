package model

const PENDING_TASK_STATE = "pending"

type StartTaskCommand struct {
	ID            string        `json:"id"`
	CodeLocation  MinIOLocation `json:"codeLocation"`
	TestsLocation MinIOLocation `json:"testsLocation"`
	Compiler      string        `json:"compiler"`
}

type TaskState struct {
	ID            string        `json:"id"`
	CodeLocation  MinIOLocation `json:"codeLocation"`
	TestsLocation MinIOLocation `json:"testsLocation"`
	Compiler      string        `json:"compiler"`
	State         string        `json:"state"`
}
