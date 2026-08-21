//go:build windows

package instance

import (
	"errors"
	"testing"
)

func TestAcquireRejectsSecondInstance(t *testing.T) {
	first, err := Acquire()
	if err != nil {
		t.Fatalf("acquire first instance: %v", err)
	}
	defer first.Close()

	second, err := Acquire()
	if second != nil {
		second.Close()
	}
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second acquisition error = %v, want ErrAlreadyRunning", err)
	}
}
