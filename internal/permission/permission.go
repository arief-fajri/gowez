package permission

// Permission is one named capability that can be granted to the UI layer
// (I8, I9).
type Permission string

// MVP permission set. New capabilities must be added here explicitly —
// there is no wildcard "all" grant (G-SEC-01).
const (
	// FSRead allows reading files through the filesystem API.
	FSRead Permission = "fs:read"
	// FSWrite allows writing files through the filesystem API.
	FSWrite Permission = "fs:write"
	// DialogOpen allows opening native file dialogs.
	DialogOpen Permission = "dialog:open"
	// Clipboard allows clipboard read/write.
	Clipboard Permission = "clipboard:readwrite"
	// WindowCtl allows window control operations (title, size, close).
	WindowCtl Permission = "window:control"
)
