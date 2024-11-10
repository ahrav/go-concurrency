package healer

import (
	"fmt"
	"testing"
	"time"
)

func TestDoWorkWithValues(t *testing.T) {
	t.Run("empty value list", func(t *testing.T) {
		done := make(chan struct{})
		doWork, stream := DoWorkWithValues[int](done)

		heartbeat := doWork(done, 50*time.Millisecond)

		select {
		case _, ok := <-stream:
			if ok {
				t.Error("expected stream to be empty")
			}
		case <-heartbeat:
			// Heartbeat received as expected
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for heartbeat")
		}
	})

	t.Run("multiple values with heartbeats", func(t *testing.T) {
		done := make(chan struct{})
		expected := []string{"a", "b", "c"}
		doWork, stream := DoWorkWithValues(done, expected...)

		heartbeat := doWork(done, 50*time.Millisecond)

		heartbeatReceived := false
		values := make([]string, 0, len(expected))
		timeout := time.After(500 * time.Millisecond)

	loop:
		for {
			select {
			case v, ok := <-stream:
				if !ok {
					break loop
				}
				values = append(values, v)
				if len(values) == len(expected) {
					break loop
				}
			case <-heartbeat:
				heartbeatReceived = true
			case <-timeout:
				t.Fatal("test timed out")
			}
		}

		if !heartbeatReceived {
			t.Error("expected at least one heartbeat")
		}

		if len(values) != len(expected) {
			t.Errorf("expected %d values, got %d", len(expected), len(values))
		}

		for i, v := range expected {
			if values[i] != v {
				t.Errorf("at index %d: expected %v, got %v", i, v, values[i])
			}
		}
	})

	t.Run("cancellation via done channel", func(t *testing.T) {
		done := make(chan struct{})
		values := []int{1, 2, 3, 4, 5}
		doWork, stream := DoWorkWithValues(done, values...)

		heartbeat := doWork(done, 50*time.Millisecond)

		time.Sleep(100 * time.Millisecond)
		close(done)

		select {
		case _, ok := <-stream:
			if ok {
				t.Error("expected stream to be closed")
			}
		case _, ok := <-heartbeat:
			if ok {
				t.Error("expected heartbeat channel to be closed")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for channels to close")
		}
	})

	t.Run("slow consumer scenario", func(t *testing.T) {
		done := make(chan struct{})
		values := []int{1, 2, 3}
		doWork, stream := DoWorkWithValues(done, values...)

		heartbeat := doWork(done, 20*time.Millisecond)

		heartbeatCount := 0
		timeout := time.After(200 * time.Millisecond)

		time.Sleep(50 * time.Millisecond) // Simulate slow consumer

	loop:
		for {
			select {
			case <-heartbeat:
				heartbeatCount++
			case <-stream:
				// Consume value
			case <-timeout:
				break loop
			}
		}

		if heartbeatCount < 2 {
			t.Errorf("expected multiple heartbeats for slow consumer, got %d", heartbeatCount)
		}
	})
}

func TestDoContinuousWork(t *testing.T) {
	t.Run("immediate cancellation", func(t *testing.T) {
		done := make(chan struct{})
		workFn := func() string {
			return "test"
		}

		doWork, stream := DoContinuousWork(done, workFn)
		heartbeat := doWork(done, 50*time.Millisecond)

		close(done)

		select {
		case _, ok := <-stream:
			if ok {
				t.Error("expected stream to be closed immediately")
			}
		case _, ok := <-heartbeat:
			if ok {
				t.Error("expected heartbeat to be closed immediately")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for channels to close")
		}
	})

	t.Run("bridge channel behavior", func(t *testing.T) {
		done := make(chan struct{})
		valueCount := 0
		workFn := func() int {
			valueCount++
			return valueCount
		}

		doWork, stream := DoContinuousWork(done, workFn)
		_ = doWork(done, 50*time.Millisecond)

		received := make(map[int]bool)
		timeout := time.After(200 * time.Millisecond)

		for i := 0; i < 5; i++ {
			select {
			case val := <-stream:
				if received[val] {
					t.Errorf("received duplicate value: %d", val)
				}
				received[val] = true
			case <-timeout:
				t.Fatal("timeout waiting for values")
			}
		}
	})
}

func TestDoStreamWork(t *testing.T) {
	t.Run("stream processing with transformation", func(t *testing.T) {
		done := make(chan struct{})
		input := make(chan int)
		workFn := func(i int) string {
			return fmt.Sprintf("processed-%d", i)
		}

		doWork, outStream := DoStreamWork(done, input, workFn)
		heartbeat := doWork(done, 50*time.Millisecond)

		go func() {
			for i := 1; i <= 3; i++ {
				input <- i
			}
			close(input)
		}()

		expected := []string{"processed-1", "processed-2", "processed-3"}
		received := make([]string, 0, len(expected))
		heartbeatReceived := false

		timeout := time.After(500 * time.Millisecond)
	loop:
		for {
			select {
			case v, ok := <-outStream:
				if !ok {
					break loop
				}
				received = append(received, v)
				if len(received) == len(expected) {
					break loop
				}
			case <-heartbeat:
				heartbeatReceived = true
			case <-timeout:
				t.Fatal("test timed out")
			}
		}

		if !heartbeatReceived {
			t.Error("expected at least one heartbeat")
		}

		if len(received) != len(expected) {
			t.Errorf("expected %d values, got %d", len(expected), len(received))
		}

		for i, v := range expected {
			if received[i] != v {
				t.Errorf("at index %d: expected %v, got %v", i, v, received[i])
			}
		}
	})

	t.Run("empty input stream", func(t *testing.T) {
		done := make(chan struct{})
		input := make(chan int)
		workFn := func(i int) string {
			return fmt.Sprintf("value-%d", i)
		}

		doWork, outStream := DoStreamWork(done, input, workFn)
		heartbeat := doWork(done, 50*time.Millisecond)

		close(input)

		select {
		case _, ok := <-outStream:
			if ok {
				t.Error("expected output stream to be closed")
			}
		case <-heartbeat:
			// Heartbeat received as expected
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for stream closure")
		}
	})

	t.Run("cancellation during processing", func(t *testing.T) {
		done := make(chan struct{})
		input := make(chan int)
		workFn := func(i int) int {
			time.Sleep(50 * time.Millisecond)
			return i
		}

		doWork, outStream := DoStreamWork(done, input, workFn)
		heartbeat := doWork(done, 20*time.Millisecond)

		go func() {
			input <- 1
			input <- 2
			time.Sleep(10 * time.Millisecond)
			close(done)
		}()

		valueReceived := false
		heartbeatReceived := false

		timeout := time.After(200 * time.Millisecond)
	loop:
		for {
			select {
			case _, ok := <-outStream:
				if !ok {
					break loop
				}
				valueReceived = true
			case <-heartbeat:
				heartbeatReceived = true
			case <-timeout:
				break loop
			}
		}

		if !valueReceived {
			t.Error("expected at least one value to be processed")
		}

		if !heartbeatReceived {
			t.Error("expected at least one heartbeat before cancellation")
		}

		select {
		case _, ok := <-outStream:
			if ok {
				t.Error("expected output stream to be closed after cancellation")
			}
		case _, ok := <-heartbeat:
			if ok {
				t.Error("expected heartbeat channel to be closed after cancellation")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for channels to close")
		}
	})
}

func TestDoParallelStreamWork(t *testing.T) {
	t.Run("parallel processing with multiple workers", func(t *testing.T) {
		done := make(chan struct{})
		input := make(chan int)
		workFn := func(i int) (string, error) {
			time.Sleep(10 * time.Millisecond)
			return fmt.Sprintf("worker-%d", i), nil
		}

		doWork, outStream := DoParallelStreamWork[int, string](done, input, workFn, 4, 20*time.Millisecond)
		heartbeat := doWork(done, 50*time.Millisecond)

		go func() {
			for i := 1; i <= 8; i++ {
				input <- i
			}
			close(input)
		}()

		results := make(map[string]bool)
		heartbeatCount := 0
		timeout := time.After(500 * time.Millisecond)

	loop:
		for {
			select {
			case result := <-outStream:
				if result.Error != nil {
					t.Errorf("unexpected error: %v", result.Error)
				}
				results[result.Value] = true
				if len(results) == 8 {
					break loop
				}
			case <-heartbeat:
				heartbeatCount++
			case <-timeout:
				t.Fatal("test timed out")
			}
		}

		if heartbeatCount == 0 {
			t.Error("expected at least one heartbeat")
		}

		if len(results) != 8 {
			t.Errorf("expected 8 results, got %d", len(results))
		}
	})

	t.Run("error handling in parallel workers", func(t *testing.T) {
		done := make(chan struct{})
		input := make(chan int)
		workFn := func(i int) (int, error) {
			if i%2 == 0 {
				return 0, fmt.Errorf("error processing %d", i)
			}
			return i * 2, nil
		}

		doWork, outStream := DoParallelStreamWork[int, int](done, input, workFn, 2, 20*time.Millisecond)
		_ = doWork(done, 50*time.Millisecond)

		go func() {
			for i := 1; i <= 4; i++ {
				input <- i
			}
			close(input)
		}()

		successCount := 0
		errorCount := 0
		timeout := time.After(300 * time.Millisecond)

	loop:
		for {
			select {
			case result := <-outStream:
				if result.Error != nil {
					errorCount++
				} else {
					successCount++
				}
				if successCount+errorCount == 4 {
					break loop
				}
			case <-timeout:
				t.Fatal("test timed out")
			}
		}

		if successCount != 2 {
			t.Errorf("expected 2 successful results, got %d", successCount)
		}
		if errorCount != 2 {
			t.Errorf("expected 2 errors, got %d", errorCount)
		}
	})

	t.Run("zero workers defaults to CPU count", func(t *testing.T) {
		done := make(chan struct{})
		input := make(chan int)
		workFn := func(i int) (string, error) {
			time.Sleep(10 * time.Millisecond)
			return fmt.Sprintf("worker-%d", i), nil
		}

		doWork, outStream := DoParallelStreamWork[int, string](done, input, workFn, 0, 20*time.Millisecond)
		heartbeat := doWork(done, 50*time.Millisecond)

		go func() {
			for i := 1; i <= 8; i++ {
				input <- i
			}
			close(input)
		}()

		results := make(map[string]bool)
		heartbeatCount := 0
		timeout := time.After(500 * time.Millisecond)

	loop:
		for {
			select {
			case result := <-outStream:
				if result.Error != nil {
					t.Errorf("unexpected error: %v", result.Error)
				}
				results[result.Value] = true
				if len(results) == 8 {
					break loop
				}
			case <-heartbeat:
				heartbeatCount++
			case <-timeout:
				t.Fatal("test timed out")
			}
		}

		if heartbeatCount == 0 {
			t.Error("expected at least one heartbeat")
		}

		if len(results) != 8 {
			t.Errorf("expected 8 results, got %d", len(results))
		}
	})

	t.Run("graceful shutdown on cancellation", func(t *testing.T) {
		done := make(chan struct{})
		input := make(chan int)
		workFn := func(i int) (int, error) {
			time.Sleep(50 * time.Millisecond)
			return i, nil
		}

		doWork, outStream := DoParallelStreamWork[int, int](done, input, workFn, 3, 20*time.Millisecond)
		heartbeat := doWork(done, 30*time.Millisecond)

		go func() {
			for i := 1; i <= 6; i++ {
				input <- i
			}
		}()

		time.Sleep(40 * time.Millisecond)
		close(done)

		select {
		case _, ok := <-outStream:
			if ok {
				t.Error("expected output stream to be closed")
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
