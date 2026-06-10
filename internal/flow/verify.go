package flow

import (
	"context"
	"fmt"

	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
)

func (r Runner) runVerification(ctx context.Context, step Step, checks []domain.Check) (domain.VerificationResult, error) {
	if r.Verifier == nil {
		return domain.VerificationResult{}, fmt.Errorf("step %q verifier is required", step.ID)
	}
	switch verifier := r.Verifier.(type) {
	case verify.Checker:
		if step.Cwd == "" {
			return verifier.Run(ctx, checks), nil
		}
		verifier.RootDir = step.Cwd
		return verifier.Run(ctx, checks), nil
	case *verify.Checker:
		if verifier == nil {
			return domain.VerificationResult{}, fmt.Errorf("step %q verifier is required", step.ID)
		}
		copy := *verifier
		if step.Cwd != "" {
			copy.RootDir = step.Cwd
		}
		return copy.Run(ctx, checks), nil
	default:
		return r.Verifier.Run(ctx, checks), nil
	}
}

func (r Runner) executeVerifyStep(ctx context.Context, step Step, stepDir string, result *Result) error {
	checks := step.Checks
	if len(checks) == 0 {
		checks = step.Verify
	}
	checks = renderChecks(checks, result)
	if len(checks) == 0 {
		return fmt.Errorf("verify step %q requires checks", step.ID)
	}
	verification, err := r.runVerification(ctx, step, checks)
	if err != nil {
		return err
	}
	result.Verification = append(result.Verification, verification)
	if err := writeParsed(stepDir, verification); err != nil {
		return err
	}
	if verification.Status != domain.VerdictPass {
		return fmt.Errorf("verify step %q failed", step.ID)
	}
	return nil
}

func (r Runner) verifyStep(ctx context.Context, step Step, result *Result) error {
	if len(step.Verify) == 0 || step.Type == StepVerify {
		return nil
	}
	verification, err := r.runVerification(ctx, step, renderChecks(step.Verify, result))
	if err != nil {
		return err
	}
	result.Verification = append(result.Verification, verification)
	if verification.Status != domain.VerdictPass {
		return fmt.Errorf("step %q verification failed", step.ID)
	}
	return nil
}

func renderChecks(checks []domain.Check, result *Result) []domain.Check {
	if len(checks) == 0 {
		return nil
	}
	rendered := make([]domain.Check, len(checks))
	for i, check := range checks {
		rendered[i] = check
		if len(check.Args) == 0 {
			continue
		}
		rendered[i].Args = make([]string, len(check.Args))
		for j, arg := range check.Args {
			rendered[i].Args[j] = renderString(arg, result)
		}
	}
	return rendered
}
