package healer

import (
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/ahrav/go-concurrency/heartbeat"
	"github.com/ahrav/go-concurrency/patterns"
)

// WorkFunc represents a function that performs work and produces values of type T.
type WorkFunc[T any] func() T

// StartGoroutineFn represents a monitored goroutine that produces heartbeats.
// It takes a done channel for cancellation and a pulse interval for heartbeat frequency.
// Returns a channel that emits heartbeat signals.
type StartGoroutineFn func(
	done <-chan struct{},
	pulseInterval time.Duration,
) (heartbeat <-chan any)

// NewSteward creates a monitoring function that supervises a goroutine and restarts it
// if it becomes unhealthy (stops sending heartbeats).
//
// Parameters:
// - timeout: Duration to wait for a heartbeat before considering the goroutine unhealthy
// - startGoroutine: The goroutine function to monitor and restart if needed
//
// Returns:
// - A StartGoroutineFn that wraps the original goroutine with monitoring capabilities
//
// The steward monitors the wrapped goroutine by expecting regular heartbeats.
// If a heartbeat isn't received within the timeout period, the steward will:
// 1. Stop the current goroutine
// 2. Start a new instance of the goroutine
// 3. Continue monitoring the new instance
func NewSteward(timeout time.Duration, startGoroutine StartGoroutineFn) StartGoroutineFn {
	return func(done <-chan struct{}, pulseInterval time.Duration) <-chan any {
		heartbeatChan := make(chan any)
		go func() {
			defer close(heartbeatChan)

			var wardDone chan struct{}
			var wardHeartbeat <-chan any
			startWard := func() {
				wardDone = make(chan struct{})
				wardHeartbeat = startGoroutine(patterns.OrDone(wardDone, done), timeout/2)
			}
			startWard()
			pulse := time.Tick(pulseInterval)

		monitorLoop:
			for {
				timeoutSignal := time.After(timeout)
				for {
					select {
					case <-pulse:
						select {
						case heartbeatChan <- struct{}{}:
						default:
						}
					case <-wardHeartbeat:
						continue monitorLoop
					case <-timeoutSignal:
						log.Println("steward: ward unhealthy; restarting")
						close(wardDone)
						startWard()
						continue monitorLoop
					case <-done:
						return
					}
				}
			}
		}()
		return heartbeatChan
	}
}

// DoWorkWithValues creates a worker that processes a slice of values and provides
// heartbeat monitoring capabilities.
//
// Parameters:
// - done: Channel for cancellation
// - values: Slice of values to process
//
// Returns:
// - StartGoroutineFn: A function that starts the monitored worker
// - <-chan T: Channel that receives processed results
//
// The worker will continuously cycle through the provided values until cancellation.
// It sends heartbeats at the specified interval and can be monitored by a steward
// for health checking and automatic restart if needed.
//
// Example:
//
//	done := make(chan struct{})
//	defer close(done)
//
//	values := []int{1, 2, 3, 4, 5}
//	worker, results := DoWorkWithValues(done, values)
//
//	// Create and start a steward to monitor the worker
//	steward := NewSteward(time.Second*30, worker)
//	heartbeats := steward(done, time.Second)
//
//	// Process results
//	for result := range results {
//	    fmt.Println("Processed:", result)
//	}
func DoWorkWithValues[T any](done <-chan struct{}, lst ...T) (StartGoroutineFn, <-chan T) {
	chanStream := make(chan (<-chan T), 1)
	stream := patterns.Bridge[T](done, chanStream)

	doWork := func(done <-chan struct{}, pulseInterval time.Duration) <-chan any {
		stream := make(chan T)
		heartbeatChan := make(chan any)
		go func() {
			defer close(stream)
			select {
			case chanStream <- stream:
			case <-done:
				return
			}

			pulse := time.Tick(pulseInterval)
			// Send initial heartbeat
			select {
			case heartbeatChan <- struct{}{}:
			default:
			}

			for {
			valueLoop:
				for _, val := range lst {
					for {
						select {
						case <-pulse:
							select {
							case heartbeatChan <- struct{}{}:
							default:
							}
						case stream <- val:
							continue valueLoop
						case <-done:
							return
						}
					}
				}
			}
		}()
		return heartbeatChan
	}
	return doWork, stream
}

// DoContinuousWork creates a worker that continuously executes a work function
// and provides heartbeat monitoring capabilities.
//
// Parameters:
// - done: Channel for cancellation
// - workFn: Function that produces values of type T
//
// Returns:
// - StartGoroutineFn: A function that starts the monitored worker
// - <-chan T: Channel that receives produced results
//
// The worker will continuously execute the work function until cancellation.
// It sends heartbeats at the specified interval and can be monitored by a steward
// for health checking and automatic restart if needed.
//
// Key differences from DoWorkWithValues:
// - Instead of cycling through a fixed set of values, it repeatedly calls a function
// - Suitable for continuous work that generates new values on each iteration
//
// Example:
//
//	done := make(chan struct{})
//	defer close(done)
//
//	// Create a work function that generates random numbers
//	workFn := func() int {
//	    return rand.Intn(100)
//	}
//
//	worker, results := DoContinuousWork(done, workFn)
//
//	// Create and start a steward to monitor the worker
//	steward := NewSteward(time.Second*30, worker)
//	heartbeats := steward(done, time.Second)
//
//	// Process results with timeout
//	timeout := time.After(5 * time.Second)
//	for {
//	    select {
//	    case r := <-results:
//	        fmt.Println("Generated:", r)
//	    case <-timeout:
//	        return
//	    case <-done:
//	        return
//	    }
//	}
func DoContinuousWork[T any](done <-chan struct{}, workFn WorkFunc[T]) (StartGoroutineFn, <-chan T) {
	chanStream := make(chan (<-chan T), 1)
	stream := patterns.Bridge[T](done, chanStream)

	doWork := func(done <-chan struct{}, pulseInterval time.Duration) <-chan any {
		stream := make(chan T)
		heartbeatChan := make(chan any)
		go func() {
			defer close(stream)
			select {
			case chanStream <- stream:
			case <-done:
				return
			}

			pulse := time.Tick(pulseInterval)
			// Send initial heartbeat
			select {
			case heartbeatChan <- struct{}{}:
			default:
			}

			for {
				select {
				case <-pulse:
					select {
					case heartbeatChan <- struct{}{}:
					default:
					}
				case stream <- workFn():
				case <-done:
					return
				}
			}
		}()
		return heartbeatChan
	}
	return doWork, stream
}

// StreamWorkFunc represents a function that processes an input value and produces an output
type StreamWorkFunc[In, Out any] func(In) Out

// DoStreamWork creates a worker that processes values from an input stream using
// the provided work function.
//
// Parameters:
// - done: Channel for cancellation
// - inStream: Channel providing input values to process
// - workFn: Function that processes each input value
//
// Returns:
// - StartGoroutineFn: A function that starts the monitored worker
// - <-chan Out: Channel that receives processed results
//
// Example:
//
//	done := make(chan struct{})
//	defer close(done)
//
//	// Create input stream
//	inputs := make(chan int)
//	go func() {
//	    defer close(inputs)
//	    for i := 1; i <= 10; i++ {
//	        inputs <- i
//	    }
//	}()
//
//	// Define work function
//	workFn := func(x int) string {
//	    return fmt.Sprintf("processed-%d", x)
//	}
//
//	worker, results := DoStreamWork(done, inputs, workFn)
//	steward := NewSteward(time.Second*30, worker)
//
//	// Process results
//	for result := range results {
//	    fmt.Println(result)
//	}
func DoStreamWork[In, Out any](
	done <-chan struct{},
	inStream <-chan In,
	workFn StreamWorkFunc[In, Out],
) (StartGoroutineFn, <-chan Out) {
	chanStream := make(chan (<-chan Out), 1)
	stream := patterns.Bridge[Out](done, chanStream)

	doWork := func(done <-chan struct{}, pulseInterval time.Duration) <-chan any {
		stream := make(chan Out)
		heartbeatChan := make(chan any)

		go func() {
			defer close(stream)

			select {
			case chanStream <- stream:
			case <-done:
				return
			}

			pulse := time.Tick(pulseInterval)
			// Send initial heartbeat
			select {
			case heartbeatChan <- struct{}{}:
			default:
			}

			for {
				select {
				case <-done:
					return
				case <-pulse:
					select {
					case heartbeatChan <- struct{}{}:
					default:
					}
				case input, ok := <-inStream:
					if !ok {
						return
					}
					// Process and send result while maintaining heartbeat.
					select {
					case stream <- workFn(input):
					case <-done:
						return
					}
				}
			}
		}()
		return heartbeatChan
	}
	return doWork, stream
}

// DoParallelStreamWork creates multiple workers that process values from an input stream
// in parallel, with continuous heartbeat monitoring during long-running operations.
//
// Parameters:
// - done: Channel for cancellation
// - inStream: Channel providing input values to process
// - workFn: Function that processes each input value and may return an error
// - numWards: Number of parallel workers (defaults to runtime.NumCPU() if <= 0)
// - pulseInterval: Interval between heartbeat signals
//
// Returns:
// - StartGoroutineFn: A function that starts the monitored workers
// - <-chan Result[Out]: Channel that receives processed results with possible errors
//
// Example:
//
//	done := make(chan struct{})
//	defer close(done)
//
//	inputs := make(chan int)
//	go func() {
//	    defer close(inputs)
//	    for i := 1; i <= 100; i++ {
//	        inputs <- i
//	    }
//	}()
//
//	workFn := func(x int) (string, error) {
//	    time.Sleep(time.Second) // Simulate long-running work
//	    return fmt.Sprintf("processed-%d", x), nil
//	}
//
//	worker, results := DoParallelStreamWork(
//	    done,
//	    inputs,
//	    workFn,
//	    4,
//	    100*time.Millisecond,
//	)
//	steward := NewSteward(time.Second*30, worker)
//
//	for result := range results {
//	    if result.Error != nil {
//	        log.Printf("Error: %v", result.Error)
//	        continue
//	    }
//	    fmt.Println(result.Value)
//	}
func DoParallelStreamWork[In, Out any](
	done <-chan struct{},
	inStream <-chan In,
	workFn func(In) (Out, error),
	numWards int,
	pulseInterval time.Duration,
) (StartGoroutineFn, <-chan heartbeat.Result[Out]) {
	// Use CPU count if numWards <= 0.
	if numWards <= 0 {
		numWards = runtime.NumCPU()
	}

	chanStream := make(chan (<-chan heartbeat.Result[Out]), 1)
	stream := patterns.Bridge[heartbeat.Result[Out]](done, chanStream)

	doWork := func(done <-chan struct{}, monitorPulseInterval time.Duration) <-chan any {
		heartbeatChan := make(chan any)

		// Create buffered channels for work distribution.
		workChan := make(chan In, numWards)
		var wg sync.WaitGroup

		// Start the input distributor.
		go func() {
			defer close(workChan)
			for input := range patterns.OrDone(done, inStream) {
				select {
				case workChan <- input:
				case <-done:
					return
				}
			}
		}()

		// Concurrently process work with multiple ward workers.
		for i := 0; i < numWards; i++ {
			wg.Add(1)
			wardStream := make(chan heartbeat.Result[Out])

			go func(wardID int) {
				defer close(wardStream)
				defer wg.Done()

				// Register ward's output channel.
				select {
				case chanStream <- wardStream:
				case <-done:
					return
				}

				// Create ward's heartbeat monitor.
				wardHeartbeat, wardResults := heartbeat.DoStreamWorkWithHeartbeatContinuous(
					done,
					pulseInterval,
					workChan,
					workFn,
				)

				// Forward ward's heartbeats to main heartbeat channel.
				go func() {
					for range patterns.OrDone(done, wardHeartbeat) {
						select {
						case heartbeatChan <- struct{}{}:
						default:
						}
					}
				}()

				// Forward ward's results.
				for result := range patterns.OrDone(done, wardResults) {
					select {
					case wardStream <- result:
					case <-done:
						return
					}
				}
			}(i)
		}

		// Close chanStream when all wards are done.
		go func() {
			wg.Wait()
			close(chanStream)
		}()

		return heartbeatChan
	}

	return doWork, stream
}
