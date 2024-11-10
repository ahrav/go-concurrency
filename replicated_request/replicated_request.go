package replicated_request

import (
	"context"
	"fmt"
	"sync"
)

// Result wraps a value and error with additional metadata.
type Result[R any] struct {
	Value     R
	Error     error
	ReplicaID int
}

// doReplicatedWork performs the same work function multiple times in parallel
// and returns the first successful result. This is useful for situations where
// you want to hedge against slow operations by running multiple instances
// simultaneously.
//
// Parameters:
// - ctx: Context for cancellation
// - numReplicas: Number of parallel work attempts to make
// - workFn: Function to replicate (should be idempotent)
//
// Returns:
// - The first successful result from any replica
// - Error if all replicas fail or context is cancelled
//
// Example:
//
//	ctx := context.Background()
//	workFn := func() (string, error) {
//	   // Simulate variable latency work
//	   delay := time.Duration(rand.Intn(100)) * time.Millisecond
//	   time.Sleep(delay)
//	   return fmt.Sprintf("completed after %v", delay), nil
//	}
//
//	result, err := doReplicatedWork(ctx, 3, workFn)
func doReplicatedWork[R any](
	ctx context.Context,
	numReplicas int,
	workFn func() (R, error),
) (R, error) {
	var wg sync.WaitGroup
	results := make(chan Result[R], 1) // Buffer of 1 is enough since we only take first result

	wg.Add(numReplicas)
	for i := 0; i < numReplicas; i++ {
		go func(replicaID int) {
			defer wg.Done()

			value, err := workFn()

			select {
			case <-ctx.Done():
				return
			case results <- Result[R]{
				Value:     value,
				Error:     err,
				ReplicaID: replicaID,
			}:
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Wait for either a result or all replicas to finish.
	select {
	case <-ctx.Done():
		var zero R
		return zero, ctx.Err()

	case result, ok := <-results:
		if !ok {
			// All replicas finished without success.
			var zero R
			return zero, fmt.Errorf("all replicas failed")
		}
		if result.Error == nil {
			// Found successful result.
			return result.Value, nil
		}
		// Got an error result, keep looking.
		for result := range results {
			if result.Error == nil {
				return result.Value, nil
			}
		}
		// If we get here, all replicas failed.
		var zero R
		return zero, fmt.Errorf("all replicas failed with errors")
	}
}
