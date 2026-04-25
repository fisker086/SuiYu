package memory

import "context"

// Nop is a no-op Provider (always used when MEMORY_PROVIDER=none or memory disabled).
type Nop struct{}

func (Nop) Retrieve(ctx context.Context, q Query) (string, error) {
	return "", nil
}

func (Nop) Record(ctx context.Context, t Turn) error {
	return nil
}
