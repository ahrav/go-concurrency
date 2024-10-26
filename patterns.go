package main

// or takes a variadic number of channels and returns a single channel that will
// close when any one of the input channels receives a value or closes.
// This pattern is useful when you want to wait for any one of a set of operations to complete,
// and making sure that you don't leak resources by waiting for all operations to complete.
//
// Example:
//
//	signal := func(after time.Duration) <-chan any {
//		c := make(chan any)
//		go func() {
//			defer close(c)
//			time.Sleep(after)
//		}()
//		return c
//	}
//
//	start := time.Now()
//
//	<-or(
//		signal(2*time.Second),
//		signal(5*time.Second),
//		signal(1*time.Second),
//		signal(3*time.Second),
//	)
//
//	fmt.Printf("done after %v", time.Since(start)) // done after 1.000..s
func or(channels ...<-chan any) <-chan any {
	switch len(channels) {
	case 0:
		return nil
	case 1:
		return channels[0]
	default:
	}

	// Create output channel that will close when any input channel signals.
	orDone := make(chan any)
	go func() {
		defer close(orDone)

		switch len(channels) {
		case 2:
			// Special case for 2 channels - simple select to avoid recursion.
			select {
			case <-channels[0]:
			case <-channels[1]:
			}
		default:
			// For 3+ channels, handle first 3 directly and recursively process the rest.
			select {
			case <-channels[0]:
			case <-channels[1]:
			case <-channels[2]:
			case <-or(append(channels[3:], orDone)...): // Recursive call for remaining channels
			}
		}
	}()
	return orDone
}

// orDone wraps a channel of type T with a done channel to enable cancellation.
// It returns a new channel that will receive all values from the input stream
// until either the stream closes, or the done channel is signaled.
// This pattern is useful for gracefully canceling channel operations
// and preventing goroutine leaks.
// This reduces the complexity at the call site and makes it easier to reason about.
//
// Example:
//
//	done := make(chan struct{})
//	nums := make(chan int)
//
//	// Start producer
//	go func() {
//		defer close(nums)
//		for i := 1; i <= 5; i++ {
//			nums <- i
//		}
//	}()
//
//	// Use orDone to handle values with cancellation
//	for val := range orDone(done, nums) {
//		fmt.Println(val)
//		if val == 3 {
//			close(done) // Cancel processing after seeing 3
//			break
//		}
//	}
func orDone[T any](done <-chan struct{}, stream <-chan T) <-chan T {
	valStream := make(chan T)
	go func() {
		defer close(valStream)
		for {
			select {
			case <-done:
				return
			case v, ok := <-stream:
				if !ok {
					return
				}
				select {
				case valStream <- v:
				case <-done:
				}
			}
		}
	}()
	return valStream
}

// tee splits a single input channel into two duplicate output channels.
// This pattern is named after the Unix tee command, which splits input into
// two identical output streams. It ensures that each value from the input
// channel is sent exactly once to each output channel, even if one consumer
// is slower than the other.
//
// Example:
//
//	done := make(chan struct{})
//	nums := make(chan int)
//
//	// Start producer
//	go func() {
//		defer close(nums)
//		for i := 1; i <= 3; i++ {
//			nums <- i
//		}
//	}()
//
//	out1, out2 := tee(done, nums)
//
//	// Both channels receive the same values
//	go func() {
//		for val := range out1 {
//			fmt.Printf("out1: %v\n", val)
//		}
//	}()
//
//	for val := range out2 {
//		fmt.Printf("out2: %v\n", val)
//	}
//
//	// Output:
//	// out1: 1
//	// out2: 1
//	// out1: 2
//	// out2: 2
//	// out1: 3
//	// out2: 3
func tee[T any](done <-chan struct{}, in <-chan T) (_, _ <-chan T) {
	out1 := make(chan T)
	out2 := make(chan T)
	go func() {
		defer close(out1)
		defer close(out2)
		for val := range orDone(done, in) {
			// Create local variables to shadow out1 and out2 to avoid blocking.
			// This ensures that each select case only attempts to send to each channel once.
			var out1, out2 = out1, out2
			for range 2 {
				select {
				case <-done:
				case out1 <- val:
					out1 = nil
				case out2 <- val:
					out2 = nil
				}
			}
		}
	}()
	return out1, out2
}
