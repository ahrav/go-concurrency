package replicated_request

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoReplicatedWork(t *testing.T) {
	t.Run("all replicas succeed", func(t *testing.T) {
		ctx := context.Background()
		workFn := func() (int, error) {
			return 42, nil
		}

		result, err := doReplicatedWork(ctx, 3, workFn)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result != 42 {
			t.Errorf("expected 42, got %v", result)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		workFn := func() (int, error) {
			time.Sleep(100 * time.Millisecond)
			return 42, nil
		}

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		_, err := doReplicatedWork(ctx, 3, workFn)

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got %v", err)
		}
	})

	t.Run("some replicas fail but one succeeds", func(t *testing.T) {
		ctx := context.Background()
		attempts := 0
		workFn := func() (int, error) {
			attempts++
			if attempts < 3 {
				return 0, errors.New("replica failed")
			}
			return 42, nil
		}

		result, err := doReplicatedWork(ctx, 3, workFn)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result != 42 {
			t.Errorf("expected 42, got %v", result)
		}
	})

	t.Run("all replicas fail", func(t *testing.T) {
		ctx := context.Background()
		workFn := func() (string, error) {
			return "", errors.New("replica failed")
		}

		_, err := doReplicatedWork(ctx, 3, workFn)

		if err == nil {
			t.Error("expected error, got nil")
		}
		if err.Error() != "all replicas failed with errors" {
			t.Errorf("expected 'all replicas failed with errors', got %v", err)
		}
	})

	t.Run("zero replicas", func(t *testing.T) {
		ctx := context.Background()
		workFn := func() (int, error) {
			return 42, nil
		}

		_, err := doReplicatedWork(ctx, 0, workFn)

		if err == nil {
			t.Error("expected error, got nil")
		}
		if err.Error() != "all replicas failed" {
			t.Errorf("expected 'all replicas failed', got %v", err)
		}
	})

	t.Run("slow successful replica", func(t *testing.T) {
		ctx := context.Background()
		workFn := func() (int, error) {
			time.Sleep(50 * time.Millisecond)
			return 42, nil
		}

		start := time.Now()
		result, err := doReplicatedWork(ctx, 5, workFn)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result != 42 {
			t.Errorf("expected 42, got %v", result)
		}
		if elapsed > 100*time.Millisecond {
			t.Errorf("operation took too long: %v", elapsed)
		}
	})
}
