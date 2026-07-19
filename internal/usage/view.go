package usage

// View is the selection state the daemon carries between renders: an empty
// PinnedProvider means aggregate mode, and WindowIndex selects an explicit
// window (index zero keeps the automatic most-constrained selection).
type View struct {
	PinnedProvider string
	WindowIndex    int
}
