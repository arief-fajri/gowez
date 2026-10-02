package permission

// Set is an explicit allow-list of permissions, built once from application
// configuration at startup.
type Set struct {
	grants map[Permission]struct{}
}

// NewSet creates a set containing exactly the given grants.
func NewSet(grants ...Permission) *Set {
	s := &Set{grants: make(map[Permission]struct{}, len(grants))}
	for _, g := range grants {
		s.grants[g] = struct{}{}
	}
	return s
}

// Allows reports whether p was explicitly granted (G-SEC-02).
func (s *Set) Allows(p Permission) bool {
	if s == nil {
		return false
	}
	_, ok := s.grants[p]
	return ok
}

// Grant adds a permission. Called from configuration loading only —
// never from a UI-reachable path (G-SEC-03).
func (s *Set) Grant(p Permission) {
	if s.grants == nil {
		s.grants = make(map[Permission]struct{})
	}
	s.grants[p] = struct{}{}
}
