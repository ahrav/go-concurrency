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
func TestOrDone(t *testing.T) {
	t.Run("stream closes before done", func(t *testing.T) {
		done := make(chan struct{})
		stream := make(chan int)
		result := orDone(done, stream)

		go func() {
			stream <- 1
			stream <- 2
			close(stream)
		}()

		var values []int
		for v := range result {
			values = append(values, v)
		}

		if len(values) != 2 || values[0] != 1 || values[1] != 2 {
			t.Errorf("expected [1 2], got %v", values)
		}
	})

	t.Run("done closes before stream", func(t *testing.T) {
		done := make(chan struct{})
		stream := make(chan int)
		result := orDone(done, stream)

		go func() {
			stream <- 1
			close(done)
			stream <- 2
			close(stream)
		}()

		count := 0
		for range result {
			count++
		}

		if count != 1 {
			t.Errorf("expected 1 value, got %d", count)
		}
	})

	t.Run("done closes while sending to valStream", func(t *testing.T) {
		done := make(chan struct{})
		stream := make(chan int)
		result := orDone(done, stream)

		go func() {
			stream <- 1
			time.Sleep(50 * time.Millisecond)
			close(done)
		}()

		// Block reading from result to simulate slow consumer
		time.Sleep(100 * time.Millisecond)
		count := 0
		for range result {
			count++
		}

		if count > 1 {
			t.Errorf("expected at most 1 value, got %d", count)
		}
	})

	t.Run("nil stream", func(t *testing.T) {
		done := make(chan struct{})
		var stream chan int
		result := orDone(done, stream)

		go func() {
			time.Sleep(50 * time.Millisecond)
			close(done)
		}()

		count := 0
		for range result {
			count++
		}

		if count != 0 {
			t.Errorf("expected 0 values from nil stream, got %d", count)
		}
	})
}

func TestTee(t *testing.T) {
	t.Run("basic tee operation", func(t *testing.T) {
		done := make(chan struct{})
		in := make(chan int)
		out1, out2 := tee(done, in)

		go func() {
			in <- 1
			in <- 2
			close(in)
		}()

		var vals1, vals2 []int
		go func() {
			for v := range out1 {
				vals1 = append(vals1, v)
			}
		}()

		for v := range out2 {
			vals2 = append(vals2, v)
		}

		if len(vals1) != 2 || len(vals2) != 2 {
			t.Errorf("expected 2 values in each output, got %d and %d", len(vals1), len(vals2))
		}
		if vals1[0] != 1 || vals1[1] != 2 || vals2[0] != 1 || vals2[1] != 2 {
			t.Errorf("expected [1 2] in both outputs, got %v and %v", vals1, vals2)
		}
	})

	t.Run("done channel closes", func(t *testing.T) {
		done := make(chan struct{})
		in := make(chan int)
		out1, out2 := tee(done, in)

		go func() {
			in <- 1
			close(done)
			in <- 2
			close(in)
		}()

		var vals1, vals2 []int
		for v := range out1 {
			vals1 = append(vals1, v)
		}
		for v := range out2 {
			vals2 = append(vals2, v)
		}

		if len(vals1) > 1 || len(vals2) > 1 {
			t.Errorf("expected at most 1 value in each output, got %d and %d", len(vals1), len(vals2))
		}
	})

	t.Run("slow consumer", func(t *testing.T) {
		done := make(chan struct{})
		in := make(chan int)
		out1, out2 := tee(done, in)

		go func() {
			in <- 1
			time.Sleep(50 * time.Millisecond)
			in <- 2
			close(in)
		}()

		var vals1, vals2 []int
		go func() {
			time.Sleep(100 * time.Millisecond)
			for v := range out1 {
				vals1 = append(vals1, v)
			}
		}()

		for v := range out2 {
			vals2 = append(vals2, v)
		}

		time.Sleep(150 * time.Millisecond)
		if len(vals1) != 2 || len(vals2) != 2 {
			t.Errorf("expected 2 values in each output, got %d and %d", len(vals1), len(vals2))
		}
	})

	t.Run("empty input channel", func(t *testing.T) {
		done := make(chan struct{})
		in := make(chan int)
		out1, out2 := tee(done, in)

		go func() {
			close(in)
		}()

		var vals1, vals2 []int
		go func() {
			for v := range out1 {
				vals1 = append(vals1, v)
			}
		}()

		for v := range out2 {
			vals2 = append(vals2, v)
		}

		if len(vals1) != 0 || len(vals2) != 0 {
			t.Errorf("expected 0 values from empty channel, got %d and %d", len(vals1), len(vals2))
		}
	})
}
func TestBridge(t *testing.T) {
	t.Run("single channel in stream", func(t *testing.T) {
		done := make(chan struct{})
		chanStream := make(chan (<-chan int))
		stream := make(chan int)

		go func() {
			chanStream <- stream
			close(stream)
			close(chanStream)
		}()

		result := bridge(done, chanStream)

		count := 0
		for range result {
			count++
		}

		if count != 0 {
			t.Errorf("expected 0 values from empty stream, got %d", count)
		}
	})

	t.Run("multiple channels with values", func(t *testing.T) {
		done := make(chan struct{})
		chanStream := make(chan (<-chan int))

		go func() {
			stream1 := make(chan int)
			stream2 := make(chan int)

			go func() {
				stream1 <- 1
				close(stream1)
			}()

			go func() {
				stream2 <- 2
				close(stream2)
			}()

			chanStream <- stream1
			chanStream <- stream2
			close(chanStream)
		}()

		result := bridge(done, chanStream)

		values := make([]int, 0)
		for v := range result {
			values = append(values, v)
		}

		if len(values) != 2 || values[0] != 1 || values[1] != 2 {
			t.Errorf("expected [1 2], got %v", values)
		}
	})

	t.Run("done cancels before processing all channels", func(t *testing.T) {
		done := make(chan struct{})
		chanStream := make(chan (<-chan int))

		go func() {
			stream1 := make(chan int)
			stream2 := make(chan int)

			go func() {
				stream1 <- 1
				close(stream1)
			}()

			chanStream <- stream1
			chanStream <- stream2
			close(done)
			stream2 <- 2
			close(stream2)
			close(chanStream)
		}()

		result := bridge(done, chanStream)

		count := 0
		for range result {
			count++
		}

		if count > 1 {
			t.Errorf("expected at most 1 value after done signal, got %d", count)
		}
	})

	t.Run("empty chanStream", func(t *testing.T) {
		done := make(chan struct{})
		chanStream := make(chan (<-chan int))

		go func() {
			close(chanStream)
		}()

		result := bridge(done, chanStream)

		count := 0
		for range result {
			count++
		}

		if count != 0 {
			t.Errorf("expected 0 values from empty chanStream, got %d", count)
		}
	})

	// TODO: Handle nil streams?
}
