package api

import (
	"context"
	"encoding/json"

	"github.com/arief-fajri/gowez/internal/permission"
)

// registerClipboard installs clipboard capabilities, gated by
// permission.Clipboard.
func registerClipboard(r *Registry) error {
	return r.Register("clipboard.readText", permission.Clipboard, func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		_ = ctx
		_ = params
		return nil, ErrNotImplemented
	})
}
