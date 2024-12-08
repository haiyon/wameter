package database

import "fmt"

// Error represents a database error
type Error struct {
	Code    string // Error code
	Message string // Error message
	Op      string // Operation that failed
	Err     error  // Original error if any
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

// NewError creates a new database error
func NewError(code string, message string, op string, err error) error {
	return &Error{
		Code:    code,
		Message: message,
		Op:      op,
		Err:     err,
	}
}
