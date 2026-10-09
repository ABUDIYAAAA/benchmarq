// Package pagination implements opaque keyset (cursor) pagination.
//
// Keyset pagination stays O(limit) regardless of how deep the client pages,
// and is stable under concurrent inserts, unlike LIMIT/OFFSET.
package pagination

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/ABUDIYAAAA/benchmarq/pkg/apperr"
	"github.com/google/uuid"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// Cursor identifies the last row of the previous page by its sort key.
type Cursor struct {
	Time time.Time `json:"t"`
	ID   uuid.UUID `json:"id"`
}

func (c Cursor) Encode() string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func Decode(raw string) (*Cursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.ID == uuid.Nil || c.Time.IsZero() {
		return nil, apperr.Invalid("malformed cursor")
	}
	return &c, nil
}

// Params are the parsed pagination inputs for a list query.
type Params struct {
	Limit int
	After *Cursor
}

// FromRequest parses ?limit= and ?cursor= query parameters.
func FromRequest(r *http.Request) (Params, error) {
	p := Params{Limit: DefaultLimit}
	q := r.URL.Query()

	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > MaxLimit {
			return p, apperr.Validation(map[string]string{"limit": "limit must be an integer between 1 and 100"})
		}
		p.Limit = n
	}
	if raw := q.Get("cursor"); raw != "" {
		c, err := Decode(raw)
		if err != nil {
			return p, apperr.Validation(map[string]string{"cursor": "cursor is invalid"})
		}
		p.After = c
	}
	return p, nil
}

// Page is a single page of results.
type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// NewPage builds a page from rows fetched with limit+1, so the presence of an
// extra row tells us whether another page exists without a COUNT query.
func NewPage[T any](rows []T, limit int, key func(T) Cursor) Page[T] {
	if rows == nil {
		rows = []T{}
	}
	if len(rows) <= limit {
		return Page[T]{Items: rows}
	}
	rows = rows[:limit]
	return Page[T]{Items: rows, NextCursor: key(rows[len(rows)-1]).Encode()}
}
