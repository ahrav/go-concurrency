package main

import (
	"errors"
	"testing"
	"time"
)

func TestDoWorkWithHeartbeat(t *testing.T) {
	t.Run("successful work with heartbeats", func(t *testing.T) {
		done := make(chan struct{})
		workFn := func() (int, error) {
			time.Sleep(100 * time.Millisecond)
			return 42, nil
		}

		heartbeat, results := doWorkWithHeartbeat[int](done, 50*time.Millisecond, workFn)

		heartbeatCount := 0
		resultReceived := false

		timeout := time.After(300 * time.Millisecond)
	loop:
		for {
			select {
			case <-heartbeat:
				heartbeatCount++
			case result := <-results:
				if result.Error != nil {
					t.Errorf("unexpected error: %v", result.Error)
				}
				if val := result.Value; val != 42 {
					t.Errorf("expected 42, got %v", result.Value)
				}
				resultReceived = true
				break loop
			case <-timeout:
				t.Error("test timed out")
				break loop
			}
		}

		if heartbeatCount == 0 {
			t.Error("expected at least one heartbeat")
		}
		if !resultReceived {
			t.Error("expected to receive result")
		}
	})

	t.Run("work with error", func(t *testing.T) {
		done := make(chan struct{})
		expectedErr := errors.New("work failed")
		workFn := func() (int, error) {
			return 0, expectedErr
		}

		_, results := doWorkWithHeartbeat[int](done, 50*time.Millisecond, workFn)

		result := <-results
		if result.Error != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, result.Error)
		}
	})

	t.Run("cancellation via done channel", func(t *testing.T) {
		done := make(chan struct{})
		workFn := func() (int, error) {
			time.Sleep(500 * time.Millisecond)
			return 42, nil
		}

		heartbeat, results := doWorkWithHeartbeat[int](done, 50*time.Millisecond, workFn)

		time.Sleep(100 * time.Millisecond)
		close(done)

		timeout := time.After(200 * time.Millisecond)
		select {
		case <-heartbeat:
			// Heartbeat received as expected
		case result, ok := <-results:
			if ok {
				t.Errorf("expected results channel to be closed, but received: %+v", result)
			}
		case <-timeout:
			t.Error("timeout reached without receiving expected signals")
		}
	})

	t.Run("rapid work completion", func(t *testing.T) {
		done := make(chan struct{})
		workFn := func() (string, error) {
			return "quick result", nil
		}

		_, results := doWorkWithHeartbeat[string](done, 1*time.Second, workFn)

		select {
		case result := <-results:
			if result.Error != nil {
				t.Errorf("unexpected error: %v", result.Error)
			}
			if val := result.Value; val != "quick result" {
				t.Errorf("expected 'quick result', got %v", result.Value)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("result not received quickly enough")
		}
	})
}
