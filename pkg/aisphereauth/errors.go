package aisphereauth

import "errors"

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNoPrincipal  = errors.New("principal not found")
)
