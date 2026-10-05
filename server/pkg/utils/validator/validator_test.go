package validator

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

type SampleDTO struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Age      int    `json:"age" validate:"min=18"`
}

func TestValidateStruct(t *testing.T) {
	// Valid struct
	valid := SampleDTO{
		Email:    "test@example.com",
		Password: "strongpassword123",
		Age:      21,
	}
	errs, ok := ValidateStruct(valid)
	if !ok || len(errs) > 0 {
		t.Fatalf("expected valid struct, got errors: %v", errs)
	}

	// Invalid struct
	invalid := SampleDTO{
		Email:    "notanemail",
		Password: "short",
		Age:      15,
	}
	errs, ok = ValidateStruct(invalid)
	if ok {
		t.Fatalf("expected invalid struct, got ok=true")
	}
	if _, exists := errs["email"]; !exists {
		t.Errorf("expected email validation error")
	}
	if _, exists := errs["password"]; !exists {
		t.Errorf("expected password validation error")
	}
}

func TestDecodeAndValidate(t *testing.T) {
	jsonBody := `{"email": "user@example.com", "password": "validpassword", "age": 25}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(jsonBody))

	var target SampleDTO
	errs, err := DecodeAndValidate(req, &target)
	if err != nil || len(errs) > 0 {
		t.Fatalf("expected successful decode and validate, got err: %v, errs: %v", err, errs)
	}

	if target.Email != "user@example.com" {
		t.Errorf("expected email to be user@example.com, got %s", target.Email)
	}
}
