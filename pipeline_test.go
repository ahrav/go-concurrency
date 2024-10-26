package main

import (
	"testing"
	"time"
)

func TestGenerator(t *testing.T) {
	t.Run("generates values until done", func(t *testing.T) {
		done := make(chan struct{})

		stream := generator(done, 1, 2, 3)

		count := 0
		for range stream {
			count++
		}

		if count != 3 {
			t.Errorf("expected 3 values, got %d", count)
		}
	})

	t.Run("stops on done signal", func(t *testing.T) {
		done := make(chan struct{})
		stream := generator(done, 1, 2, 3)
		close(done)

		count := 0
		for range stream {
			count++
		}

		if count > 1 {
			t.Error("generator didn't stop on done signal")
		}
	})
}

func TestMultiply(t *testing.T) {
	t.Run("multiplies values correctly", func(t *testing.T) {
		done := make(chan struct{})
		input := make(chan int, 3)
		input <- 1
		input <- 2
		input <- 3
		close(input)

		result := multiply(done, input, 2)

		expected := []int{2, 4, 6}
		i := 0
		for v := range result {
			if v != expected[i] {
				t.Errorf("expected %d, got %d", expected[i], v)
			}
			i++
		}
	})

	t.Run("stops on done signal", func(t *testing.T) {
		done := make(chan struct{})
		input := make(chan int)
		result := multiply(done, input, 2)

		go func() {
			input <- 1
			time.Sleep(10 * time.Millisecond)
			close(done)
		}()

		count := 0
		for range result {
			count++
		}

		if count > 1 {
			t.Error("multiply didn't stop on done signal")
		}
	})
}

func TestAdd(t *testing.T) {
	t.Run("adds values correctly", func(t *testing.T) {
		done := make(chan struct{})
		input := make(chan int, 3)
		input <- 1
		input <- 2
		input <- 3
		close(input)

		result := add(done, input, 5)

		expected := []int{6, 7, 8}
		i := 0
		for v := range result {
			if v != expected[i] {
				t.Errorf("expected %d, got %d", expected[i], v)
			}
			i++
		}
	})

	t.Run("stops on done signal", func(t *testing.T) {
		done := make(chan struct{})
		input := make(chan int)
		result := add(done, input, 5)

		go func() {
			input <- 1
			time.Sleep(10 * time.Millisecond)
			close(done)
		}()

		count := 0
		for range result {
			count++
		}

		if count > 1 {
			t.Error("add didn't stop on done signal")
		}
	})
}
func TestPipelineComposition(t *testing.T) {
	t.Run("combines generator, multiply, and add stages", func(t *testing.T) {
		done := make(chan struct{})

		// Create pipeline: generate -> multiply by 2 -> add 5
		result := add(done, multiply(done, generator(done, 1, 2, 3), 2), 5)

		// Expected values after multiply by 2 and add 5:
		// 1 -> 2 -> 7
		// 2 -> 4 -> 9
		// 3 -> 6 -> 11
		expected := []int{7, 9, 11}
		i := 0
		for v := range result {
			if v != expected[i] {
				t.Errorf("expected %d, got %d at position %d", expected[i], v, i)
			}
			i++
		}

		if i != len(expected) {
			t.Errorf("expected %d values, got %d", len(expected), i)
		}
	})
}

func TestRepeat(t *testing.T) {
	t.Run("repeats single value continuously until done", func(t *testing.T) {
		done := make(chan struct{})
		stream := repeat(done, 42)

		values := make([]int, 3)
		for i := 0; i < 3; i++ {
			values[i] = <-stream
		}

		for i, v := range values {
			if v != 42 {
				t.Errorf("expected 42 at position %d, got %d", i, v)
			}
		}
		close(done)
	})

	t.Run("repeats multiple values in sequence", func(t *testing.T) {
		done := make(chan struct{})
		stream := repeat(done, 1, 2, 3)

		expected := []int{1, 2, 3, 1, 2, 3}
		actual := make([]int, 6)

		for i := 0; i < 6; i++ {
			actual[i] = <-stream
		}
		close(done)

		for i, v := range actual {
			if v != expected[i] {
				t.Errorf("expected %d at position %d, got %d", expected[i], i, v)
			}
		}
	})

	t.Run("works with string type", func(t *testing.T) {
		done := make(chan struct{})
		stream := repeat(done, "hello", "world")

		expected := []string{"hello", "world", "hello", "world"}
		actual := make([]string, 4)

		for i := 0; i < 4; i++ {
			actual[i] = <-stream
		}
		close(done)

		for i, v := range actual {
			if v != expected[i] {
				t.Errorf("expected %s at position %d, got %s", expected[i], i, v)
			}
		}
	})

	t.Run("stops immediately on done signal", func(t *testing.T) {
		done := make(chan struct{})
		stream := repeat(done, 1, 2, 3)

		<-stream // read one value
		close(done)

		time.Sleep(10 * time.Millisecond)
		select {
		case _, ok := <-stream:
			if ok {
				t.Error("channel should be closed after done signal")
			}
		default:
			t.Error("channel should be closed, not blocking")
		}
	})

	t.Run("handles empty values slice", func(t *testing.T) {
		done := make(chan struct{})
		stream := repeat(done, []int{}...)

		select {
		case <-stream:
			t.Error("expected no values for empty input")
		case <-time.After(10 * time.Millisecond):
			// Success - no values received
		}
		close(done)
	})
}

func TestRepeatFn(t *testing.T) {
	t.Run("generates values from function continuously until done", func(t *testing.T) {
		done := make(chan struct{})
		counter := 0
		fn := func() any {
			counter++
			return counter
		}
		stream := repeatFn(done, fn)

		values := make([]any, 3)
		for i := 0; i < 3; i++ {
			values[i] = <-stream
		}

		for i, v := range values {
			if v != i+1 {
				t.Errorf("expected %d at position %d, got %d", i+1, i, v)
			}
		}
		close(done)
	})

	t.Run("handles nil return from function", func(t *testing.T) {
		done := make(chan struct{})
		fn := func() any {
			return nil
		}
		stream := repeatFn(done, fn)

		value := <-stream
		if value != nil {
			t.Errorf("expected nil, got %v", value)
		}
		close(done)
	})

	t.Run("handles panic in function", func(t *testing.T) {
		done := make(chan struct{})
		fn := func() any {
			panic("test panic")
		}
		stream := repeatFn(done, fn)

		<-stream
		close(done)
	})

	t.Run("stops immediately after done signal", func(t *testing.T) {
		done := make(chan struct{})
		callCount := 0
		fn := func() any {
			callCount++
			return callCount
		}
		stream := repeatFn(done, fn)

		<-stream // read one value
		close(done)

		time.Sleep(10 * time.Millisecond)
		finalCount := callCount
		time.Sleep(10 * time.Millisecond)

		if callCount > finalCount {
			t.Error("function continued to be called after done signal")
		}
	})

	t.Run("handles different types", func(t *testing.T) {
		done := make(chan struct{})
		types := []any{"string", 42, true, 3.14}
		currentIndex := 0
		fn := func() any {
			value := types[currentIndex]
			currentIndex = (currentIndex + 1) % len(types)
			return value
		}
		stream := repeatFn(done, fn)

		for i := 0; i < len(types); i++ {
			value := <-stream
			if value != types[i] {
				t.Errorf("expected %v, got %v", types[i], value)
			}
		}
		close(done)
	})
}

func TestTake(t *testing.T) {
	t.Run("takes specified number of values from stream", func(t *testing.T) {
		done := make(chan struct{})
		valueStream := make(chan int, 5)
		for i := 1; i <= 5; i++ {
			valueStream <- i
		}
		close(valueStream)

		result := take(done, valueStream, 3)

		count := 0
		values := make([]int, 0, 3)
		for v := range result {
			values = append(values, v)
			count++
		}

		if count != 3 {
			t.Errorf("expected 3 values, got %d", count)
		}

		expected := []int{1, 2, 3}
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("at position %d: expected %d, got %d", i, expected[i], v)
			}
		}
	})

	t.Run("handles take count larger than stream", func(t *testing.T) {
		done := make(chan struct{})
		valueStream := make(chan int, 2)
		valueStream <- 1
		valueStream <- 2
		close(valueStream)

		result := take(done, valueStream, 5)

		count := 0
		for range result {
			count++
		}

		if count != 2 {
			t.Errorf("expected 2 values, got %d", count)
		}
	})

	t.Run("handles zero take count", func(t *testing.T) {
		done := make(chan struct{})
		valueStream := make(chan int, 2)
		valueStream <- 1
		valueStream <- 2
		close(valueStream)

		result := take(done, valueStream, 0)

		count := 0
		for range result {
			count++
		}

		if count != 0 {
			t.Errorf("expected 0 values, got %d", count)
		}
	})

	t.Run("stops on done signal before completing", func(t *testing.T) {
		done := make(chan struct{})
		valueStream := make(chan int)

		go func() {
			for i := 1; i <= 5; i++ {
				valueStream <- i
			}
		}()

		result := take(done, valueStream, 3)

		<-result // take first value
		close(done)

		count := 1
		for range result {
			count++
		}

		if count > 2 {
			t.Errorf("take didn't stop on done signal, got %d values", count)
		}
	})

	t.Run("works with string type", func(t *testing.T) {
		done := make(chan struct{})
		valueStream := make(chan string, 3)
		valueStream <- "a"
		valueStream <- "b"
		valueStream <- "c"
		close(valueStream)

		result := take(done, valueStream, 2)

		values := make([]string, 0, 2)
		for v := range result {
			values = append(values, v)
		}

		expected := []string{"a", "b"}
		if len(values) != len(expected) {
			t.Errorf("expected %d values, got %d", len(expected), len(values))
		}
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("at position %d: expected %s, got %s", i, expected[i], v)
			}
		}
	})
}

func TestPipelineCompositionII(t *testing.T) {
	t.Run("repeat and take composition", func(t *testing.T) {
		done := make(chan struct{})
		values := take(done, repeat(done, 1, 2, 3), 4)

		expected := []int{1, 2, 3, 1}
		i := 0
		for v := range values {
			if v != expected[i] {
				t.Errorf("expected %d, got %d at position %d", expected[i], v, i)
			}
			i++
		}
		if i != 4 {
			t.Errorf("expected 4 values, got %d", i)
		}
	})

	t.Run("repeat, multiply, and take composition", func(t *testing.T) {
		done := make(chan struct{})
		pipeline := take(
			done,
			multiply(done, repeat(done, 1, 2, 3), 2),
			3,
		)

		expected := []int{2, 4, 6}
		i := 0
		for v := range pipeline {
			if v != expected[i] {
				t.Errorf("expected %d, got %d at position %d", expected[i], v, i)
			}
			i++
		}
	})

	t.Run("repeatFn and take composition", func(t *testing.T) {
		done := make(chan struct{})
		counter := 0
		fn := func() any {
			counter++
			return counter
		}

		pipeline := take(done, repeatFn(done, fn), 3)

		expected := []int{1, 2, 3}
		i := 0
		for v := range pipeline {
			if v.(int) != expected[i] {
				t.Errorf("expected %d, got %d at position %d", expected[i], v, i)
			}
			i++
		}
	})

	t.Run("complex pipeline composition", func(t *testing.T) {
		done := make(chan struct{})
		pipeline := take(
			done,
			add(
				done,
				multiply(done, repeat(done, 1, 2, 3), 2),
				1,
			),
			4,
		)

		expected := []int{3, 5, 7, 3}
		i := 0
		for v := range pipeline {
			if v != expected[i] {
				t.Errorf("expected %d, got %d at position %d", expected[i], v, i)
			}
			i++
		}
	})
}
