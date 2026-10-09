package database

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// Postgres SQLSTATE codes we react to.
const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
	codeCheckViolation      = "23514"
)

func pgError(err error) (*pgconn.PgError, bool) {
	var pgErr *pgconn.PgError
	ok := errors.As(err, &pgErr)
	return pgErr, ok
}

// IsUniqueViolation reports whether err is a unique violation. When constraint
// is non-empty, it must also match the violated constraint (or index) name.
func IsUniqueViolation(err error, constraint string) bool {
	return matches(err, codeUniqueViolation, constraint)
}

// IsForeignKeyViolation reports whether err is a foreign key violation.
func IsForeignKeyViolation(err error, constraint string) bool {
	return matches(err, codeForeignKeyViolation, constraint)
}

// IsCheckViolation reports whether err is a CHECK constraint violation.
func IsCheckViolation(err error, constraint string) bool {
	return matches(err, codeCheckViolation, constraint)
}

// ConstraintName returns the violated constraint, if err is a Postgres error.
func ConstraintName(err error) string {
	if pgErr, ok := pgError(err); ok {
		return pgErr.ConstraintName
	}
	return ""
}

func matches(err error, code, constraint string) bool {
	pgErr, ok := pgError(err)
	if !ok || pgErr.Code != code {
		return false
	}
	return constraint == "" || pgErr.ConstraintName == constraint
}
