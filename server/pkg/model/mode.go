package model

import (
	"time"
	"uuid"
)

type Model struct {
	ID        uuid.UUID
	UpdatedAt time.Time
	DeletedAt time.Time
}
