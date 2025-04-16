package model

const (
	CompilingTaskState = "compiling"
	TestingTaskState   = "testing"
	CompletedTaskState = "completed"
)

type StartTaskCommand struct {
	ID            string       `json:"id"`
	CodeLocation  FileLocation `json:"codeLocation"`
	TestsLocation FileLocation `json:"testsLocation"`
	Compiler      string       `json:"compiler"`
}

type Task struct {
	ID                 string       `json:"id"`
	CodeLocation       FileLocation `json:"codeLocation"`
	TestsLocation      FileLocation `json:"testsLocation"`
	ExecutableLocation FileLocation `json:"executableLocation"`
	Compiler           string       `json:"compiler"`
	State              string       `json:"state"`
	TestsResults       []TestResult `json:"testsResults"`
}
