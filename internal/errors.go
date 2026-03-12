package internal

import "fmt"

type ExitError struct {
	Code int
	Err  error
}

func NewExitError(code int, err error) *ExitError {
	return &ExitError{Code: code, Err: err}
}

func (e *ExitError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit %d", e.Code)
}

type StageError struct {
	Stage   string
	Err     error
	Journal string
}

func NewStageError(stage string, err error) *StageError {
	return &StageError{Stage: stage, Err: err}
}

func (e *StageError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("stage %s: %v", e.Stage, e.Err)
	}
	return fmt.Sprintf("stage %s failed", e.Stage)
}
