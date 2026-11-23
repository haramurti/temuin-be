// Package tx exposes a transaction boundary used by application services.
package tx

import "context"

// Manager runs fn inside a single database transaction. Repositories pick the
// transaction up from the context passed to fn.
type Manager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
