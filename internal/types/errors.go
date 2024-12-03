package types

import "errors"

var (
	ErrAgentNotFound = errors.New("agent not found")
	ErrInvalidDriver = errors.New("invalid storage driver")
)
