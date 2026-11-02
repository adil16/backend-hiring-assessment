package domain

import "errors"

var (
	ErrForbidden    = errors.New("forbidden")
	ErrUserNotFound = errors.New("user not found")
)
