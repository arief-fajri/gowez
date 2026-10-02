package api

import (
	"context"
	"encoding/json"
)

// registerWindow installs window-control capabilities, gated by
// permission.WindowCtl.
func registerWindow(r *Registry) error {
	return r.Register("window.setTitle", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		_ = ctx
		_ = params
		return nil, ErrNotImplemented
	})
}
