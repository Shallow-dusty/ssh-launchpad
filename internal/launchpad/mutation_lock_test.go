package launchpad

import (
	"context"
	"testing"
)

func TestMutationLockBlocksConcurrentMutation(t *testing.T) {
	ctx, release, err := mutationContext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, nested, err := mutationContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nested()
	// A Windows mutex is thread-affine and reentrant on its owning thread.
	// Exercise a competing execution thread, not recursive OS acquisition.
	result := make(chan error, 1)
	go func() {
		unlock, err := acquireMutationLock()
		if err == nil {
			unlock()
		}
		result <- err
	}()
	if err := <-result; err == nil {
		t.Fatal("a concurrent mutation was accepted")
	}
}

func TestMutationLockCanBeReacquired(t *testing.T) {
	for i := 0; i < 2; i++ {
		release, err := acquireMutationLock()
		if err != nil {
			t.Fatal(err)
		}
		release()
	}
}
