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
