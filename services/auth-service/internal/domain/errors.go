package domain

import "errors"

var (
	// ErrTokenStoreNotReady is returned when a request provides an API key but the token store
	// has not been loaded yet.
	ErrTokenStoreNotReady = errors.New("token store not ready")
	// ErrInvalidAPIKey is returned when an API key is provided but not found in the token store.
	ErrInvalidAPIKey = errors.New("invalid api key")
)
