package orchestration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/bocacorazon/dft/internal/adapters/agentstub"
	"github.com/bocacorazon/dft/internal/adapters/verify"
	"github.com/bocacorazon/dft/internal/domain"
	"github.com/bocacorazon/dft/internal/ports"
)

func TestMacroOrchestratorRunsFullLocalIncrementLifecycle(t *testing.T) {
	root := t.TempDir()
	git := &macroRecordingGit{defaultBranch: "main"}
	orchestrator := MacroOrchestrator{
		Agent:        agentstub.Adapter{},
		Worktrees:    WorktreeManager{Git: git, WorktreeRoot: filepath.Join(root, ".dft", "worktrees")},
		Verifier:     verify.Checker{RootDir: root},
		ArtifactRoot: root,
		Review:       domain.ReviewDecision{Approved: true},
	}

	result, err := orchestrator.Execute(context.Background(), domain.DemandPackage{
		ID:        "run-123",
		Title:     "Macro orchestrator",
		RawDemand: "Macro orchestrator",
		AcceptanceCriteria: []string{
			"Full local increment lifecycle completes.",
		},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Increment.Branch != "increment/run-123" {
		t.Fatalf("increment branch = %q, want increment/run-123", result.Increment.Branch)
	}
	if result.Evaluation.Status != domain.VerdictPass {
		t.Fatalf("evaluation = %#v, want pass", result.Evaluation)
	}
	if len(git.merges) != 2 {
		t.Fatalf("merge count = %d, want spec merge and final merge", len(git.merges))
	}
	if got := git.merges[1]; got != (ports.MergeRequest{Source: "increment/run-123", Target: "main"}) {
		t.Fatalf("final merge = %#v", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".dft", "runs", "run-123", "macro-result.json")); err != nil {
		t.Fatalf("macro result artifact missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".dft", "runs", "run-123", "eval-plan.json")); err != nil {
		t.Fatalf("eval plan artifact missing: %v", err)
	}
}

func TestMacroOrchestratorCanHoldPassingIncrementBeforeDefaultMerge(t *testing.T) {
	root := t.TempDir()
	git := &macroRecordingGit{defaultBranch: "main"}
	orchestrator := MacroOrchestrator{
		Agent:         agentstub.Adapter{},
		Worktrees:     WorktreeManager{Git: git, WorktreeRoot: filepath.Join(root, ".dft", "worktrees")},
		Verifier:      verify.Checker{RootDir: root},
		ArtifactRoot:  root,
		Review:        domain.ReviewDecision{Approved: true},
		HoldIncrement: true,
	}

	result, err := orchestrator.Execute(context.Background(), domain.DemandPackage{
		ID:                 "run-123",
		Title:              "Macro orchestrator",
		RawDemand:          "Macro orchestrator",
		AcceptanceCriteria: []string{"Full local increment lifecycle completes."},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IncrementHeld {
		t.Fatalf("IncrementHeld = false, want true")
	}
	if len(git.merges) != 1 {
		t.Fatalf("merge count = %d, want only spec merge with held increment", len(git.merges))
	}
}

func TestMacroOrchestratorWritesFixPlanAndSkipsFinalMergeOnFailedEval(t *testing.T) {
	root := t.TempDir()
	git := &macroRecordingGit{defaultBranch: "main"}
	orchestrator := MacroOrchestrator{
		Agent:        agentstub.Adapter{},
		Worktrees:    WorktreeManager{Git: git, WorktreeRoot: filepath.Join(root, ".dft", "worktrees")},
		Verifier:     failingVerifier{},
		ArtifactRoot: root,
		Review:       domain.ReviewDecision{Approved: true},
	}

	result, err := orchestrator.Execute(context.Background(), domain.DemandPackage{
		ID:        "run-123",
		Title:     "Macro orchestrator",
		RawDemand: "Macro orchestrator",
		AcceptanceCriteria: []string{
			"Full local increment lifecycle completes.",
		},
	})

	if err == nil {
		t.Fatalf("Execute returned nil error, want failed evaluation")
	}
	if result.WBSAmendment == nil {
		t.Fatalf("WBS amendment = nil, want remediation plan")
	}
	if len(git.merges) != 1 {
		t.Fatalf("merge count = %d, want only spec merge before failed eval", len(git.merges))
	}
	if _, err := os.Stat(filepath.Join(root, ".dft", "runs", "run-123", "fix-plan", "wbs-amendment.json")); err != nil {
		t.Fatalf("fix plan artifact missing: %v", err)
	}
}

func TestMacroOrchestratorRetriesRemediationSpecsAfterFailedEval(t *testing.T) {
	root := t.TempDir()
	git := &macroRecordingGit{defaultBranch: "main"}
	orchestrator := MacroOrchestrator{
		Agent:          agentstub.Adapter{},
		Worktrees:      WorktreeManager{Git: git, WorktreeRoot: filepath.Join(root, ".dft", "worktrees")},
		Verifier:       &failThenPassVerifier{},
		ArtifactRoot:   root,
		Review:         domain.ReviewDecision{Approved: true},
		MaxEvalRetries: 1,
	}

	result, err := orchestrator.Execute(context.Background(), domain.DemandPackage{
		ID:                 "run-123",
		Title:              "Macro orchestrator",
		RawDemand:          "Macro orchestrator",
		AcceptanceCriteria: []string{"Full local increment lifecycle completes."},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Evaluation.Status != domain.VerdictPass {
		t.Fatalf("evaluation = %#v, want pass after remediation", result.Evaluation)
	}
	if result.WBSAmendment == nil {
		t.Fatalf("WBS amendment = nil, want remediation record")
	}
	if len(git.merges) != 3 {
		t.Fatalf("merge count = %d, want spec merge, remediation merge, final merge", len(git.merges))
	}
}

func TestMacroOrchestratorRetriesRemediationSpecsAfterBlockedReview(t *testing.T) {
	root := t.TempDir()
	git := &macroRecordingGit{defaultBranch: "main"}
	agent := &reviewFailThenPassAgent{}
	orchestrator := MacroOrchestrator{
		Agent:          agent,
		Worktrees:      WorktreeManager{Git: git, WorktreeRoot: filepath.Join(root, ".dft", "worktrees")},
		Verifier:       verify.Checker{RootDir: root},
		ArtifactRoot:   root,
		MaxEvalRetries: 1,
	}

	result, err := orchestrator.Execute(context.Background(), domain.DemandPackage{
		ID:                 "run-123",
		Title:              "Macro orchestrator",
		RawDemand:          "Macro orchestrator",
		AcceptanceCriteria: []string{"Full local increment lifecycle completes."},
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.Review.Approved {
		t.Fatalf("review approved = false, want true after remediation")
	}
	if result.WBSAmendment == nil {
		t.Fatalf("WBS amendment = nil, want review remediation record")
	}
	if agent.reviews != 2 {
		t.Fatalf("review calls = %d, want initial review and retry", agent.reviews)
	}
	if len(git.merges) != 3 {
		t.Fatalf("merge count = %d, want spec merge, remediation merge, final merge", len(git.merges))
	}
}

func TestMacroOrchestratorWritesInboxItemWhenFinalReviewBlocks(t *testing.T) {
	root := t.TempDir()
	git := &macroRecordingGit{defaultBranch: "main"}
	orchestrator := MacroOrchestrator{
		Agent:        agentstub.Adapter{},
		Worktrees:    WorktreeManager{Git: git, WorktreeRoot: filepath.Join(root, ".dft", "worktrees")},
		Verifier:     verify.Checker{RootDir: root},
		ArtifactRoot: root,
		Review: domain.ReviewDecision{
			Approved: false,
			Findings: []domain.Finding{{
				Message: "review finding",
			}},
		},
	}

	result, err := orchestrator.Execute(context.Background(), domain.DemandPackage{
		ID:        "run-123",
		Title:     "Macro orchestrator",
		RawDemand: "Macro orchestrator",
		AcceptanceCriteria: []string{
			"Full local increment lifecycle completes.",
		},
	})

	if err == nil {
		t.Fatalf("Execute returned nil error, want blocked review")
	}
	if result.Review.Approved {
		t.Fatalf("review approved = true, want false")
	}
	if len(git.merges) != 1 {
		t.Fatalf("merge count = %d, want only spec merge before review block", len(git.merges))
	}
	if _, err := os.Stat(filepath.Join(root, ".dft", "inbox", "review-blocked-run-123.json")); err != nil {
		t.Fatalf("inbox review block missing: %v", err)
	}
}

type macroRecordingGit struct {
	defaultBranch string
	merges        []ports.MergeRequest
}

func (g *macroRecordingGit) DefaultBranch(context.Context) (string, error) {
	return g.defaultBranch, nil
}

func (g *macroRecordingGit) CreateBranch(context.Context, ports.CreateBranchRequest) error {
	return nil
}

func (g *macroRecordingGit) CreateWorktree(context.Context, ports.CreateWorktreeRequest) error {
	return nil
}

func (g *macroRecordingGit) Merge(_ context.Context, request ports.MergeRequest) error {
	g.merges = append(g.merges, request)
	return nil
}

type failingVerifier struct{}

func (failingVerifier) Run(context.Context, []domain.Check) domain.VerificationResult {
	return domain.VerificationResult{
		Status: domain.VerdictFail,
		Findings: []domain.Finding{{
			CheckID: "forced-failure",
			Message: "forced failure",
		}},
	}
}

type failThenPassVerifier struct {
	calls int
}

func (v *failThenPassVerifier) Run(context.Context, []domain.Check) domain.VerificationResult {
	v.calls++
	if v.calls == 1 {
		return domain.VerificationResult{
			Status: domain.VerdictFail,
			Findings: []domain.Finding{{
				CheckID: "forced-failure",
				Message: "forced failure",
			}},
		}
	}
	return domain.VerificationResult{Status: domain.VerdictPass}
}

type reviewFailThenPassAgent struct {
	reviews int
}

func (a *reviewFailThenPassAgent) Invoke(ctx context.Context, request ports.AgentRequest) (ports.AgentResponse, error) {
	if request.AgentName != "dft-review.agent.md" {
		return agentstub.Adapter{}.Invoke(ctx, request)
	}
	a.reviews++
	if a.reviews == 1 {
		return marshalAgentResponse(domain.ReviewDecision{
			Approved: false,
			Findings: []domain.Finding{{
				Message: "review finding",
			}},
		})
	}
	return marshalAgentResponse(domain.ReviewDecision{Approved: true})
}

func marshalAgentResponse(value any) (ports.AgentResponse, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return ports.AgentResponse{}, err
	}
	return ports.AgentResponse{Raw: string(content)}, nil
}
