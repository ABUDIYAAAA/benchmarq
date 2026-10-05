package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	validate *validator.Validate
	once     sync.Once
)

// GetValidator returns the singleton validator instance.
func GetValidator() *validator.Validate {
	once.Do(func() {
		validate = validator.New()
	})
	return validate
}

// FormatValidationErrors turns go-playground validator errors into user-friendly field-to-message map.
func FormatValidationErrors(err error) map[string]string {
	fieldErrors := make(map[string]string)
	var valErrors validator.ValidationErrors

	if errors.As(err, &valErrors) {
		for _, fieldErr := range valErrors {
			fieldName := toSnakeCase(fieldErr.Field())
			switch fieldErr.Tag() {
			case "required":
				fieldErrors[fieldName] = fmt.Sprintf("%s is required", fieldName)
			case "email":
				fieldErrors[fieldName] = fmt.Sprintf("%s must be a valid email address", fieldName)
			case "min":
				fieldErrors[fieldName] = fmt.Sprintf("%s must be at least %s characters long", fieldName, fieldErr.Param())
			case "max":
				fieldErrors[fieldName] = fmt.Sprintf("%s cannot exceed %s characters", fieldName, fieldErr.Param())
			case "uuid":
				fieldErrors[fieldName] = fmt.Sprintf("%s must be a valid UUID", fieldName)
			case "oneof":
				fieldErrors[fieldName] = fmt.Sprintf("%s must be one of [%s]", fieldName, fieldErr.Param())
			default:
				fieldErrors[fieldName] = fmt.Sprintf("%s failed validation rule '%s'", fieldName, fieldErr.Tag())
			}
		}
		return fieldErrors
	}

	fieldErrors["error"] = err.Error()
	return fieldErrors
}

// ValidateStruct validates a struct using tags.
func ValidateStruct(s any) (map[string]string, bool) {
	v := GetValidator()
	err := v.Struct(s)
	if err == nil {
		return nil, true
	}
	return FormatValidationErrors(err), false
}

// DecodeAndValidate reads JSON body from request and validates the target struct.
func DecodeAndValidate(r *http.Request, target any) (map[string]string, error) {
	if r.Body == nil {
		return map[string]string{"body": "request body is empty"}, errors.New("empty body")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return map[string]string{"body": "request body is empty"}, err
		}
		return map[string]string{"body": fmt.Sprintf("invalid JSON payload: %s", err.Error())}, err
	}

	if errs, ok := ValidateStruct(target); !ok {
		return errs, errors.New("validation failed")
	}

	return nil, nil
}

// toSnakeCase converts PascalCase/CamelCase struct field names to snake_case.
func toSnakeCase(s string) string {
	var res strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			res.WriteRune('_')
		}
		res.WriteRune(r)
	}
	return strings.ToLower(res.String())
}
