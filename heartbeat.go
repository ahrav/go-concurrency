package main

import "time"

// Result is a generic struct that holds a value of any type and an error.
//
// Fields:
// - Value: The value of type T.
// - Error: An error that occurred during the operation.
type Result[T any] struct {
	Value T
	Error error
}

// doWorkWithHeartbeat performs work and sends periodic heartbeat signals.
//
// Parameters:
// - done: A read-only channel of type struct{} used to signal when to stop the work.
// - pulseInterval: A time.Duration specifying the interval between heartbeat signals.
// - workFn: A function that performs the work and returns a value of any type and an error.
//
// Returns:
// - A read-only channel of type struct{} that sends heartbeat signals.
// - A read-only channel of type Result[T] that sends the results of the work.
//
// Example:
//
//	done := make(chan struct{})
//	defer close(done)
//
//	// Define work function that simulates processing
//	workFn := func() (any, error) {
//		// Simulate work that takes 2 seconds
//		time.Sleep(2 * time.Second)
//		return "task completed", nil
//	}
//
//	// Start work with heartbeat every 500ms
//	heartbeat, results := doWorkWithHeartbeat[string](done, 500*time.Millisecond, workFn)
//
//	// Monitor heartbeats and results
//	for {
//		select {
//		case <-heartbeat:
//			fmt.Println("pulse")
//		case r := <-results:
//			if r.Error != nil {
//				fmt.Printf("error: %v\n", r.Error)
//				return
//			}
//			fmt.Printf("result: %v\n", r.Value)
//			return
//		}
//	}
//
//	// Output:
//	// pulse
//	// pulse
//	// pulse
//	// pulse
//	// result: task completed
func doWorkWithHeartbeat[T any](
	done <-chan struct{},
	pulseInterval time.Duration,
	workFn func() (T, error),
) (<-chan struct{}, <-chan Result[T]) {
	heartbeat := make(chan struct{})
	results := make(chan Result[T])

	go func() {
		defer close(heartbeat)
		defer close(results)

		// Create a ticker that sends the current time on the channel after each pulseInterval.
		pulse := time.Tick(pulseInterval)

		// sendPulse sends a heartbeat signal if the heartbeat channel is not blocked.
		sendPulse := func() {
			select {
			case heartbeat <- struct{}{}:
			default:
			}
		}

		// sendResult sends the result of the work function to the results channel.
		// It continues to send heartbeat signals until the result is sent or
		// the done channel is closed.
		sendResult := func(r Result[T]) {
			for {
				select {
				case <-done:
					return
				case <-pulse:
					sendPulse()
				case results <- r:
					return
				}
			}
		}

		// Main loop that performs the work and sends heartbeat signals.
		for {
			select {
			case <-done:
				return
			case <-pulse:
				sendPulse()
			default:
				// Execute the actual work.
				val, err := workFn()
				sendResult(Result[T]{Value: val, Error: err})
			}
		}
	}()
	return heartbeat, results
}
