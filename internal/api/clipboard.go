package api

import (
	"context"
	"encoding/json"
)

// registerClipboard installs clipboard capabilities, gated by
// permission.Clipboard.
func registerClipboard(r *Registry) error {
	return r.Register("clipboard.readText", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		_ = ctx
		_ = params
		return nil, ErrNotImplemented
	})
}
