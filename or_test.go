package main

import (
	"testing"
	"time"
)

func TestOr(t *testing.T) {
	t.Run("empty channels", func(t *testing.T) {
		result := or()
		if result != nil {
			t.Error("expected nil for empty channels")
		}
	})

	t.Run("single channel", func(t *testing.T) {
		ch := make(chan any)
		result := or(ch)
		if result != ch {
			t.Error("expected same channel to be returned for single input")
		}
	})

	t.Run("two channels - first signals", func(t *testing.T) {
		ch1 := make(chan any)
		ch2 := make(chan any)
		result := or(ch1, ch2)

		go func() {
			close(ch1)
		}()

		select {
		case <-result:
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for channel signal")
		}
	})

	t.Run("two channels - second signals", func(t *testing.T) {
		ch1 := make(chan any)
		ch2 := make(chan any)
		result := or(ch1, ch2)

		go func() {
			close(ch2)
		}()

		select {
		case <-result:
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for channel signal")
		}
	})

	t.Run("multiple channels - middle signals", func(t *testing.T) {
		ch1 := make(chan any)
		ch2 := make(chan any)
		ch3 := make(chan any)
		ch4 := make(chan any)
		result := or(ch1, ch2, ch3, ch4)

		go func() {
			close(ch2)
		}()

		select {
		case <-result:
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for channel signal")
		}
	})

	t.Run("multiple channels - last signals", func(t *testing.T) {
		ch1 := make(chan any)
		ch2 := make(chan any)
		ch3 := make(chan any)
		ch4 := make(chan any)
		result := or(ch1, ch2, ch3, ch4)

		go func() {
			close(ch4)
		}()

		select {
		case <-result:
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for channel signal")
		}
	})
}
