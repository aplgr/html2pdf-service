package domain

import (
	"errors"
	"testing"
)

func TestDomainErrors_AreStableAndUsableWithErrorsIs(t *testing.T) {
	if ErrTokenStoreNotReady == nil {
		t.Fatalf("ErrTokenStoreNotReady must not be nil")
	}
	if ErrInvalidAPIKey == nil {
		t.Fatalf("ErrInvalidAPIKey must not be nil")
	}

	if ErrTokenStoreNotReady == ErrInvalidAPIKey {
		t.Fatalf("domain errors must be distinct")
	}

	wrappedNotReady := errors.Join(errors.New("context"), ErrTokenStoreNotReady)
	if !errors.Is(wrappedNotReady, ErrTokenStoreNotReady) {
		t.Fatalf("expected errors.Is to match ErrTokenStoreNotReady")
	}

	wrappedInvalid := errors.Join(errors.New("context"), ErrInvalidAPIKey)
	if !errors.Is(wrappedInvalid, ErrInvalidAPIKey) {
		t.Fatalf("expected errors.Is to match ErrInvalidAPIKey")
	}

	if got := ErrTokenStoreNotReady.Error(); got == "" {
		t.Fatalf("ErrTokenStoreNotReady message should not be empty")
	}
	if got := ErrInvalidAPIKey.Error(); got == "" {
		t.Fatalf("ErrInvalidAPIKey message should not be empty")
	}
}
