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

		// Launch work in separate goroutine to prevent blocking heartbeats.
		workComplete := make(chan Result[T])
		go func() {
			defer close(workComplete)
			// Create a new done channel specific to this work.
			workDone := make(chan struct{})
			go func() {
				select {
				case <-done:
					close(workDone)
				default:
				}
			}()

			select {
			case <-workDone:
				return
			case workComplete <- func() Result[T] {
				val, err := workFn()
				return Result[T]{Value: val, Error: err}
			}():
			}
		}()

		// Main loop that handles heartbeats and work completion.
		for {
			select {
			case <-done:
				return
			case <-pulse:
				sendPulse()
			case result := <-workComplete:
				sendResult(result)
				return
			}
		}
	}()
	return heartbeat, results
}

// doStreamWorkWithHeartbeatPerUnitOfWork performs work on a stream of inputs with a heartbeat signal
// sent at the start of each unit of work.
//
// Parameters:
// - done: A read-only channel to signal cancellation
// - inStream: Input channel providing values to process
// - workFn: Function that processes each input value
//
// Returns:
// - A heartbeat channel that signals when work starts
// - A results channel that provides processed values
//
// Example:
//
//	done := make(chan struct{})
//	defer close(done)
//
//	inputs := make(chan int)
//	go func() {
//		defer close(inputs)
//		for i := 1; i <= 3; i++ {
//			inputs <- i
//		}
//	}()
//
//	workFn := func(i int) (string, error) {
//		time.Sleep(100 * time.Millisecond)
//		return fmt.Sprintf("processed %d", i), nil
//	}
//
//	heartbeat, results := doStreamWorkWithHeartbeatPerUnitOfWork[int, string](
//		done,
//		inputs,
//		workFn,
//	)
func doStreamWorkWithHeartbeatPerUnitOfWork[T any, R any](
	done <-chan struct{},
	inStream <-chan T,
	workFn func(T) (R, error),
) (<-chan struct{}, <-chan Result[R]) {
	heartbeatStream := make(chan struct{}, 1)
	rsultsStream := make(chan Result[R])

	go func() {
		defer close(heartbeatStream)
		defer close(rsultsStream)

		for val := range orDone(done, inStream) {
			workComplete := make(chan Result[R])

			go func(v T) {
				workComplete <- func() Result[R] {
					res, err := workFn(v)
					return Result[R]{Value: res, Error: err}
				}()
			}(val)

			// Send a heartbeat signal before starting work on each unit of work.
			// We handle this seperately in the event there is nobody
			// listening to the heartbeat channel.
			select {
			case heartbeatStream <- struct{}{}:
			default:
			}

			select {
			case <-done:
				return
			case result := <-workComplete:
				select {
				case <-done:
					return
				case rsultsStream <- result:
				}
			}
		}
	}()

	return heartbeatStream, rsultsStream
}

// doStreamWorkWithHeartbeatContinuous performs work on a stream of inputs with continuous
// heartbeat signals throughout the entire processing of each work item.
//
// Parameters:
// - done: A read-only channel to signal cancellation
// - pulseInterval: The interval between heartbeat signals
// - inStream: Input channel providing values to process
// - workFn: Function that processes each input value
//
// Returns:
// - A heartbeat channel that signals continuously during work
// - A results channel that provides processed values
//
// Key difference from PerUnitOfWork version:
// This version maintains continuous heartbeats during the entire processing of each work item,
// making it more suitable for long-running operations where you need constant health monitoring.
//
// Example:
//
//	done := make(chan struct{})
//	defer close(done)
//
//	inputs := make(chan int)
//	go func() {
//		defer close(inputs)
//		for i := 1; i <= 3; i++ {
//			inputs <- i
//		}
//	}()
//
//	workFn := func(i int) (string, error) {
//		time.Sleep(1 * time.Second)  // Long running work
//		return fmt.Sprintf("processed %d", i), nil
//	}
//
//	heartbeat, results := doStreamWorkWithHeartbeatContinuous[int, string](
//		done,
//		100*time.Millisecond,
//		inputs,
//		workFn,
//	)
func doStreamWorkWithHeartbeatContinuous[T any, R any](
	done <-chan struct{},
	pulseInterval time.Duration,
	inStream <-chan T,
	workFn func(T) (R, error),
) (<-chan struct{}, <-chan Result[R]) {
	heartbeatStream := make(chan struct{})
	resultsStream := make(chan Result[R])

	go func() {
		defer close(heartbeatStream)
		defer close(resultsStream)

		pulse := time.Tick(pulseInterval)

		sendPulse := func() {
			select {
			case heartbeatStream <- struct{}{}:
			default:
			}
		}

		// sendResult sends the result while maintaining heartbeat.
		sendResult := func(r Result[R]) {
			for {
				select {
				case <-done:
					return
				case <-pulse:
					sendPulse()
				case resultsStream <- r:
					return
				}
			}
		}

	nextValue:
		for val := range orDone(done, inStream) {
			workComplete := make(chan Result[R])

			// Launch work in separate goroutine.
			go func(v T) {
				workComplete <- func() Result[R] {
					res, err := workFn(v)
					return Result[R]{Value: res, Error: err}
				}()
			}(val)

			// Keep sending heartbeats until work completes.
			for {
				select {
				case <-done:
					return
				case <-pulse:
					sendPulse()
				case result := <-workComplete:
					sendResult(result)
					continue nextValue
				}
			}
		}
	}()

	return heartbeatStream, resultsStream
}
