package logsql

import (
	"fmt"
	"net/http"
)

type TranslationError struct {
	Code    int
	Message string
	Err     error
}

func (e *TranslationError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *TranslationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newBadRequest(msg string, err error) *TranslationError {
	return &TranslationError{
		Code:    http.StatusBadRequest,
		Message: msg,
		Err:     err,
	}
}
