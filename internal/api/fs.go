package api

import (
	"context"
	"encoding/json"

	"github.com/arief-fajri/gowez/internal/permission"
)

// registerFS installs filesystem capabilities. Every handler must be gated
// by permission.FSRead / permission.FSWrite before execution (G-SEC-03).
func registerFS(r *Registry) error {
	if err := r.Register("fs.readTextFile", permission.FSRead, func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		_ = ctx
		_ = params
		return nil, ErrNotImplemented
	}); err != nil {
		return err
	}
	return r.Register("fs.writeTextFile", permission.FSWrite, func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		_ = ctx
		_ = params
		return nil, ErrNotImplemented
	})
}
