package viewid

import (
	"testing"

	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

// typesOf types ep's function as the DXIL pipeline does before Analyze.
// Many tests here build a function whose expressions name globals,
// functions or locals the module does not declare, to drive one path of
// the taint walk; such a function has no types, and typesOf returns nil,
// which the analysis reads as no expression having a type.
func typesOf(t *testing.T, m *ir.Module, ep *ir.EntryPoint) *typifier.Typifier {
	t.Helper()
	types, err := typifier.Function(m, &ep.Function)
	if err != nil {
		return nil
	}
	return types
}
