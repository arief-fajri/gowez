package api

import (
	"context"
	"encoding/json"
)

// registerDialog installs native dialog capabilities, gated by
// permission.DialogOpen.
func registerDialog(r *Registry) error {
	return r.Register("dialog.open", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		_ = ctx
		_ = params
		return nil, ErrNotImplemented
	})
}
