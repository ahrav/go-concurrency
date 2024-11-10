package heartbeat

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
		workFn := func() (int, error) { return 0, expectedErr }

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

func TestDoStreamWorkWithHeartbeatPerUnitOfWork(t *testing.T) {
	t.Run("processes multiple inputs successfully", func(t *testing.T) {
		done := make(chan struct{})
		inputs := make(chan int)
		expected := []int{2, 4, 6}

		go func() {
			defer close(inputs)
			for _, v := range []int{1, 2, 3} {
				inputs <- v
			}
		}()

		workFn := func(i int) (int, error) { return i * 2, nil }

		heartbeat, results := doStreamWorkWithHeartbeatPerUnitOfWork[int, int](done, inputs, workFn)

		var received []int
		heartbeatReceived := false

		cnt := 0
		for cnt < len(expected) {
			select {
			case result := <-results:
				if result.Error != nil {
					t.Errorf("unexpected error: %v", result.Error)
				}
				received = append(received, result.Value)
				cnt++
			case <-heartbeat:
				heartbeatReceived = true
			case <-time.After(100 * time.Millisecond):
				t.Fatal("timeout reached without receiving expected signals")
			}
		}

		if !heartbeatReceived {
			t.Error("expected at least one heartbeat")
		}

		if len(received) != len(expected) {
			t.Errorf("expected %d results, got %d", len(expected), len(received))
		}

		for i, v := range expected {
			if received[i] != v {
				t.Errorf("expected %d, got %d at position %d", v, received[i], i)
			}
		}
	})
}

func TestDoStreamWorkWithHeartbeatContinuous(t *testing.T) {
	t.Run("maintains continuous heartbeat during long operations", func(t *testing.T) {
		done := make(chan struct{})
		inputs := make(chan string)
		heartbeatCount := 0

		go func() {
			defer close(inputs)
			inputs <- "test"
		}()

		workFn := func(s string) (string, error) {
			time.Sleep(300 * time.Millisecond)
			return s + "_processed", nil
		}

		heartbeat, results := doStreamWorkWithHeartbeatContinuous[string, string](done, 50*time.Millisecond, inputs, workFn)

		timeout := time.After(400 * time.Millisecond)
		resultReceived := false

	loop:
		for {
			select {
			case <-heartbeat:
				heartbeatCount++
			case result := <-results:
				if result.Error != nil {
					t.Errorf("unexpected error: %v", result.Error)
				}
				if result.Value != "test_processed" {
					t.Errorf("expected 'test_processed', got %v", result.Value)
				}
				resultReceived = true
				break loop
			case <-timeout:
				t.Error("test timed out")
				break loop
			}
		}

		if heartbeatCount < 3 {
			t.Errorf("expected at least 3 heartbeats during long operation, got %d", heartbeatCount)
		}
		if !resultReceived {
			t.Error("expected to receive result")
		}
	})

	t.Run("handles errors in work function", func(t *testing.T) {
		done := make(chan struct{})
		inputs := make(chan int)
		expectedErr := errors.New("processing error")

		go func() {
			defer close(inputs)
			inputs <- 1
		}()

		workFn := func(i int) (int, error) { return 0, expectedErr }

		_, results := doStreamWorkWithHeartbeatContinuous[int, int](done, 50*time.Millisecond, inputs, workFn)

		select {
		case result := <-results:
			if result.Error != expectedErr {
				t.Errorf("expected error %v, got %v", expectedErr, result.Error)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for error result")
		}
	})

	t.Run("handles empty input stream", func(t *testing.T) {
		done := make(chan struct{})
		inputs := make(chan int)
		close(inputs)

		workFn := func(i int) (int, error) { return i * 2, nil }

		heartbeat, results := doStreamWorkWithHeartbeatContinuous[int, int](done, 50*time.Millisecond, inputs, workFn)

		select {
		case _, ok := <-results:
			if ok {
				t.Error("expected results channel to be closed")
			}
		case _, ok := <-heartbeat:
			if ok {
				t.Error("expected heartbeat channel to be closed")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for channels to close")
		}
	})
}
