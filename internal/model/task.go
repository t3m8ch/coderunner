package model

const (
	CompilingTaskState = "compiling"
	TestingTaskState   = "testing"
)

type StartTaskCommand struct {
	ID            string        `json:"id"`
	CodeLocation  MinIOLocation `json:"codeLocation"`
	TestsLocation MinIOLocation `json:"testsLocation"`
	Compiler      string        `json:"compiler"`
}

type Task struct {
	ID                 string        `json:"id"`
	CodeLocation       MinIOLocation `json:"codeLocation"`
	TestsLocation      MinIOLocation `json:"testsLocation"`
	ExecutableLocation MinIOLocation `json:"executableLocation"`
	Compiler           string        `json:"compiler"`
	State              string        `json:"state"`
}
