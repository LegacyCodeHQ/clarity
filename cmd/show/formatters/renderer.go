package formatters

// Renderer translates a Scene's primitives into a concrete output syntax (DOT,
// Mermaid, ...). It is the contract that keeps the formatters in lockstep: every
// render primitive is a method here, so adding or removing one is a compile
// error in any formatter that has not kept up — enforced by the static
// assertions below. A Renderer must contain only syntax, never graph logic.
//
// The contract grows one primitive at a time as logic migrates out of the
// formatters; today it covers the graph header and trailer.
type Renderer interface {
	// Begin emits the graph header/preamble from the resolved header primitive.
	Begin(GraphHeader)
	// Finish emits any trailer and returns the complete output.
	Finish() (string, error)
}

// Render walks a Scene through a Renderer. As primitives migrate into the Scene,
// this becomes the single rendering path shared by every formatter.
func Render(scene Scene, r Renderer) (string, error) {
	r.Begin(scene.Header)
	return r.Finish()
}

// Compile-time guard: both formatters must satisfy the full Renderer contract.
// Growing Renderer with a new primitive breaks the build here until DOT and
// Mermaid both implement it — that is the drift protection.
var (
	_ Renderer = (*dotRenderer)(nil)
	_ Renderer = (*mermaidRenderer)(nil)
)
