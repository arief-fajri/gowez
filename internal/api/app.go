package api

import (
	"context"
	"encoding/json"
)

// infoResult is the app.getInfo payload. ipcVersion is kept equal to
// ipc.Version by a cross-package assertion in internal/ipc (api must not
// import ipc — the dependency runs ipc → api).
const infoResult = `{"name":"gowez","ipcVersion":1,"engine":"goja"}`

// registerApp installs non-native introspection methods. They carry no
// permission (they read no OS state), which is exactly what makes them the
// M4 success-path proof: UI → IPC → Go API → result, end to end, without
// touching the OS (checklist: "UI can invoke Go API", "Go can return
// success").
func registerApp(r *Registry) error {
	return r.Register("app.getInfo", "", func(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
		_ = ctx
		_ = params
		return json.RawMessage(infoResult), nil
	})
}
