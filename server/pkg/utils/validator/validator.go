package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/go-playground/validator/v10"
)

var (
	validate *validator.Validate
	once     sync.Once

	slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

// GetValidator returns the singleton validator instance.
func GetValidator() *validator.Validate {
	once.Do(func() {
		validate = validator.New()

		// Report errors using JSON field names so clients see the keys they sent.
		validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name, _, _ := strings.Cut(fld.Tag.Get("json"), ",")
			if name == "-" {
				return ""
			}
			return name
		})

		_ = validate.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
			return slugPattern.MatchString(fl.Field().String())
		})
	})
	return validate
}

// FormatValidationErrors turns go-playground validator errors into user-friendly field-to-message map.
func FormatValidationErrors(err error) map[string]string {
	fieldErrors := make(map[string]string)
	var valErrors validator.ValidationErrors

	if errors.As(err, &valErrors) {
		for _, fieldErr := range valErrors {
			fieldName := fieldPath(fieldErr)
			switch fieldErr.Tag() {
			case "required":
				fieldErrors[fieldName] = fmt.Sprintf("%s is required", fieldName)
			case "email":
				fieldErrors[fieldName] = fmt.Sprintf("%s must be a valid email address", fieldName)
			case "min", "gte":
				fieldErrors[fieldName] = fmt.Sprintf("%s must be at least %s%s", fieldName, fieldErr.Param(), unitSuffix(fieldErr))
			case "max", "lte":
				fieldErrors[fieldName] = fmt.Sprintf("%s cannot exceed %s%s", fieldName, fieldErr.Param(), unitSuffix(fieldErr))
			case "gt":
				fieldErrors[fieldName] = fmt.Sprintf("%s must be greater than %s", fieldName, fieldErr.Param())
			case "lt":
				fieldErrors[fieldName] = fmt.Sprintf("%s must be less than %s", fieldName, fieldErr.Param())
			case "uuid":
				fieldErrors[fieldName] = fmt.Sprintf("%s must be a valid UUID", fieldName)
			case "url", "http_url":
				fieldErrors[fieldName] = fmt.Sprintf("%s must be a valid URL", fieldName)
			case "slug":
				fieldErrors[fieldName] = fmt.Sprintf("%s may only contain lowercase letters, digits and single hyphens", fieldName)
			case "oneof":
				fieldErrors[fieldName] = fmt.Sprintf("%s must be one of [%s]", fieldName, fieldErr.Param())
			case "unique":
				fieldErrors[fieldName] = fmt.Sprintf("%s must not contain duplicates", fieldName)
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

// fieldPath returns the JSON path of the failing field relative to the root
// struct, e.g. "options[1].content".
//
// Intermediate segments still spelled as Go identifiers (embedded structs,
// which have no JSON name) are dropped, since they do not appear in the payload.
func fieldPath(fe validator.FieldError) string {
	parts := strings.Split(fe.Namespace(), ".")[1:]
	kept := make([]string, 0, len(parts))
	for i, p := range parts {
		if i < len(parts)-1 && p != "" && unicode.IsUpper(rune(p[0])) {
			continue
		}
		kept = append(kept, p)
	}
	return toSnakeCase(strings.Join(kept, "."))
}

// unitSuffix clarifies whether a length bound refers to characters or items.
func unitSuffix(fe validator.FieldError) string {
	switch fe.Kind() {
	case reflect.String:
		return " characters"
	case reflect.Slice, reflect.Array, reflect.Map:
		return " items"
	default:
		return ""
	}
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
