package ports

import (
	"context"

	"github.com/bocacorazon/dft/internal/domain"
)

// Verifier executes deterministic verification checks.
type Verifier interface {
	Run(context.Context, []domain.Check) domain.VerificationResult
}
