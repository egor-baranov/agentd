package control

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("conflict")
	ErrInvalidTransition = errors.New("invalid transition")
	ErrUnsupportedAgent  = errors.New("unsupported agent")
	ErrUnsupportedBundle = errors.New("unsupported bundle")
	ErrUnsupportedAuth   = errors.New("unsupported auth mode")
	ErrNoCapacity        = errors.New("no schedulable capacity")
	ErrContainerNotReady = errors.New("container not ready")
	ErrSessionNotActive  = errors.New("session not active")
)
