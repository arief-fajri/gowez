package layout

import (
	"errors"

	"github.com/volantisfrontend/gowez/internal/ui"
)

// ErrNotImplemented is returned until the layout engine lands (Milestone 2).
var ErrNotImplemented = errors.New("layout: not implemented yet (Milestone 2)")

// Geometry is the layout result for one node.
type Geometry struct {
	// Border is the border box in viewport coordinates.
	Border Box
	// Content is the content box in viewport coordinates.
	Content Box
}

// Layout computes geometry for the tree rooted at root and returns it keyed
// by node ID.
//
// Same input must always yield the same boxes (Module 6: layout produces
// deterministic geometry). Nodes with DisplayNone produce no entry.
func Layout(root *ui.Node) (map[ui.NodeID]Geometry, error) {
	if root == nil {
		return nil, errors.New("layout: nil root")
	}
	// TODO(M2): block + flex algorithms over the style-resolved tree.
	return nil, ErrNotImplemented
}
