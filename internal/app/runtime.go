package app

import (
	"fmt"

	"github.com/arief-fajri/gowez/internal/api"
	"github.com/arief-fajri/gowez/internal/ipc"
	"github.com/arief-fajri/gowez/internal/observe"
	"github.com/arief-fajri/gowez/internal/permission"
	"github.com/arief-fajri/gowez/internal/script"
)

// newRuntime builds the IPC dispatcher and the embedded JS engine — the
// "JS runtime" step of the startup sequence (docs/PLATFORM.md: config →
// window → renderer → JS runtime → scene → ready).
//
// Wiring order matters: the dispatcher registers the native registry
// (single door to the OS, G-SEC-01) behind an empty grant set — deny by
// default (G-SEC-02); permissioned methods become reachable only when the
// M6 permission model loads grants. Both components share the metrics
// recorder and reporter so every IPC call and JS evaluation is observable
// (P5).
func newRuntime(metrics *observe.Recorder, reporter observe.Reporter) (*ipc.Dispatcher, script.Engine, error) {
	disp := ipc.NewDispatcher()
	disp.SetObservation(metrics, reporter)
	disp.SetGrants(permission.NewSet())

	reg, err := api.NewDefaultRegistry()
	if err != nil {
		return nil, nil, fmt.Errorf("native api registry: %w", err)
	}
	if err := disp.RegisterRegistry(reg); err != nil {
		return nil, nil, fmt.Errorf("registry wiring: %w", err)
	}

	eng, err := script.New(script.DefaultLimits, disp)
	if err != nil {
		return nil, nil, fmt.Errorf("js engine: %w", err)
	}
	eng.SetObservation(metrics, reporter)
	return disp, eng, nil
}
