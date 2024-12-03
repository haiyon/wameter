package validator

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	once     sync.Once
	validate *validator.Validate
)

// Validator represents a validator instance
type Validator struct {
	validate *validator.Validate
}

// New creates a new validator instance
func New() *Validator {
	once.Do(func() {
		validate = validator.New()

		// Register custom validation functions
		validate.RegisterValidation("mac", validateMAC)
		validate.RegisterValidation("hostname", validateHostname)

		// Register custom type functions
		validate.RegisterCustomTypeFunc(validateTime, TimeType{})

		// Use JSON tag names in error messages
		validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
	})

	return &Validator{
		validate: validate,
	}
}

// Struct validates a struct
func (v *Validator) Struct(s any) error {
	if err := v.validate.Struct(s); err != nil {
		if _, ok := err.(*validator.InvalidValidationError); ok {
			return fmt.Errorf("invalid validation error: %w", err)
		}

		var errMsgs []string
		for _, err := range err.(validator.ValidationErrors) {
			errMsgs = append(errMsgs, formatError(err))
		}
		return fmt.Errorf("validation failed: %s", strings.Join(errMsgs, "; "))
	}
	return nil
}

// Var validates a single variable
func (v *Validator) Var(field any, tag string) error {
	return v.validate.Var(field, tag)
}

// Engine returns the underlying validator engine
func (v *Validator) Engine() any {
	return v.validate
}

// formatError formats a validation error
func formatError(err validator.FieldError) string {
	field := err.Field()
	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s", field, err.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s", field, err.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email", field)
	case "ip":
		return fmt.Sprintf("%s must be a valid IP address", field)
	case "mac":
		return fmt.Sprintf("%s must be a valid MAC address", field)
	case "hostname":
		return fmt.Sprintf("%s must be a valid hostname", field)
	default:
		return fmt.Sprintf("%s failed on tag %s", field, err.Tag())
	}
}

// Custom validation functions
func validateMAC(fl validator.FieldLevel) bool {
	mac := fl.Field().String()
	if mac == "" {
		return true
	}

	parts := strings.Split(mac, ":")
	if len(parts) != 6 {
		return false
	}

	for _, part := range parts {
		if len(part) != 2 {
			return false
		}
	}

	return true
}

// Custom validation functions
func validateHostname(fl validator.FieldLevel) bool {
	hostname := fl.Field().String()
	if hostname == "" {
		return true
	}

	// Basic hostname validation
	if len(hostname) > 255 {
		return false
	}

	for _, part := range strings.Split(hostname, ".") {
		if len(part) > 63 {
			return false
		}
	}

	return true
}

// TimeType is a placeholder type for time validation
type TimeType struct{}

// ValidateTime returns the value as is
func validateTime(field reflect.Value) any {
	return field.Interface()
}
