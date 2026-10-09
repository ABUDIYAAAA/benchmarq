// Package request contains small helpers for decoding HTTP input.
package request

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// MaxBodyBytes caps JSON request bodies. Question prompts and coding test
// cases can be large, so this is more generous than a typical API.
const MaxBodyBytes = 2 << 20 // 2 MiB

// URLParamUUID parses a chi path parameter as a UUID.
func URLParamUUID(r *http.Request, name string) (uuid.UUID, error) {
	raw := chi.URLParam(r, name)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperr.Validation(map[string]string{name: fmt.Sprintf("%s must be a valid UUID", name)})
	}
	return id, nil
}

// ReadJSONObject reads the request body and ensures it is a single JSON object.
// The raw bytes are returned so callers can apply them as a merge patch.
func ReadJSONObject(w http.ResponseWriter, r *http.Request) (json.RawMessage, error) {
	if r.Body == nil {
		return nil, apperr.Validation(map[string]string{"body": "request body is empty"})
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, apperr.Validation(map[string]string{"body": "request body is too large"})
		}
		return nil, apperr.Validation(map[string]string{"body": "failed to read request body"})
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, apperr.Validation(map[string]string{"body": "request body is empty"})
	}
	if trimmed[0] != '{' || !json.Valid(trimmed) {
		return nil, apperr.Validation(map[string]string{"body": "request body must be a JSON object"})
	}
	return trimmed, nil
}

// ApplyMergePatch decodes patch on top of dst, following JSON merge-patch
// semantics (RFC 7386): absent keys are left untouched, present keys overwrite,
// and null clears pointer/slice/map fields. Unknown keys are rejected.
func ApplyMergePatch(dst any, patch json.RawMessage) error {
	if len(patch) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(patch))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.Validation(map[string]string{"body": fmt.Sprintf("invalid JSON payload: %s", err.Error())})
	}
	if dec.More() {
		return apperr.Validation(map[string]string{"body": "request body must contain a single JSON object"})
	}
	return nil
}
