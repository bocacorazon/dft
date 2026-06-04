package hermes

import (
	"testing"

	"github.com/bocacorazon/dft/internal/ports"
)

func TestHermesAdapter_ImplementsInterfaces(t *testing.T) {
	var _ ports.AgentAdapter = Adapter{}
	var _ ports.CommandDispatcher = Adapter{}
	// Just verifies that it compiles the requirements.
}
