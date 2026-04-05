package tbm

import (
	"errors"
	"mydb/src/main/backend/utils"
)


var (
	ErrInvalidValues   = errors.New("Invalid values.")
	ErrInvalidLogOP    = errors.New("Invalid logic operation.")
	ErrNoThatField     = errors.New("No that field.")
	ErrFieldHasNoField = errors.New("Field has no index.")
)

type entry map[string]interface{}

type table struct {
	TBM      *tableManager
	SelfUUID utils.UUID

	Name   string
	status byte
	Next   utils.UUID
	fields []*field
}



