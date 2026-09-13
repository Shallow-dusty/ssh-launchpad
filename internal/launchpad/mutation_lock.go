package launchpad

import "context"

type mutationLockKey struct{}

// The lock spans the authoritative re-plan and all mutations/recovery. Helpers
// acquire it themselves; callers never hold it while waiting for elevation.
func mutationContext(ctx context.Context) (context.Context, func(), error) {
	if ctx.Value(mutationLockKey{}) != nil {
		return ctx, func() {}, nil
	}
	release, err := acquireMutationLock()
	if err != nil {
		return ctx, nil, err
	}
	return context.WithValue(ctx, mutationLockKey{}, true), release, nil
}
