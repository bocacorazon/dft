package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bocacorazon/dft/internal/domain"
)

const incrementPackageSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "dft:increment-package",
  "title": "IncrementPackage",
  "description": "Contract output of the Intent phase. A refined, verifiable increment ready for solution design.",
  "type": "object",
  "required": ["id", "title", "raw_increment", "refined_increment", "acceptance_criteria", "verified_complete"],
  "properties": {
    "id": {"type": "string", "description": "Human-readable slug derived from the increment title (e.g., extract-design-phase-agents)"},
    "title": {"type": "string", "description": "Concise title for this increment"},
    "raw_increment": {"type": "string", "description": "Original increment text as received or self-generated"},
    "refined_increment": {"type": "string", "description": "Clear, unambiguous restatement of the increment"},
    "acceptance_criteria": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["id", "description"],
        "properties": {
          "id": {"type": "string", "description": "Unique criterion ID (e.g., AC-001)"},
          "description": {"type": "string", "description": "Observable, testable behavior that must hold true"},
          "requirement": {"type": "string", "description": "Optional link to a requirement ID (e.g., REQ-001)"}
        }
      }
    },
    "assumptions": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Assumptions made during refinement"
    },
    "non_goals": {
      "type": "array",
      "items": {"type": "string"},
      "description": "Things explicitly out of scope"
    },
    "ambiguities": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["id", "description"],
        "properties": {
          "id": {"type": "string"},
          "description": {"type": "string"},
          "resolution": {"type": "string"}
        }
      }
    },
    "verified_complete": {"type": "boolean", "description": "Must be true for the increment to be actionable"},
    "created_at": {"type": "string", "format": "date-time"}
  }
}`

// schemaJSON contains the JSON Schema for each contract type, derived from Go structs.
var schemaJSON = map[string]string{
	"increment-package": incrementPackageSchema,
	"solution-design": `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "dft:solution-design",
  "title": "SolutionDesign",
  "description": "Contract output of the Solution phase. Complete design ready for execution orchestration.",
  "type": "object",
  "required": ["increment_package_id", "test_plan", "wbs", "lane_assignments", "eval_surface_contract"],
  "properties": {
    "increment_package_id": {"type": "string", "description": "Matches the id from the increment-package"},
    "test_plan": {
      "type": "object",
      "required": ["increment_package_id", "scenarios"],
      "properties": {
        "increment_package_id": {"type": "string"},
        "requirement_ids": {"type": "array", "items": {"type": "string"}},
        "scenarios": {
          "type": "array",
          "minItems": 1,
          "items": {
            "type": "object",
            "required": ["id", "name", "then"],
            "properties": {
              "id": {"type": "string"},
              "name": {"type": "string"},
              "requirement_ids": {"type": "array", "items": {"type": "string"}},
              "description": {"type": "string"},
              "given": {"type": "array", "items": {"type": "string"}},
              "when": {"type": "array", "items": {"type": "string"}},
              "then": {"type": "array", "items": {"type": "string"}, "minItems": 1}
            }
          }
        }
      }
    },
    "wbs": {
      "type": "object",
      "required": ["increment_package_id", "specs"],
      "properties": {
        "increment_package_id": {"type": "string"},
        "specs": {
          "type": "array",
          "minItems": 1,
          "items": {
            "type": "object",
            "required": ["id", "acceptance_criteria"],
            "properties": {
              "id": {"type": "string"},
              "description": {"type": "string"},
              "prompt_path": {"type": "string"},
              "acceptance_criteria": {"type": "array", "items": {"type": "string"}, "minItems": 1}
            }
          }
        }
      }
    },
    "lane_assignments": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["spec_id", "lane"],
        "properties": {
          "spec_id": {"type": "string"},
          "lane": {"type": "string", "description": "Executor lane: speckit, stub, or direct"},
          "rationale": {"type": "string"}
        }
      }
    },
    "eval_surface_contract": {
      "type": "object",
      "required": ["increment_package_id", "surfaces"],
      "properties": {
        "increment_package_id": {"type": "string"},
        "surfaces": {
          "type": "array",
          "minItems": 1,
          "items": {
            "type": "object",
            "required": ["id", "kind", "artifact_ref", "adapter_family", "environment_class"],
            "properties": {
              "id": {"type": "string"},
              "kind": {"type": "string", "enum": ["cli", "http_api", "graphql", "grpc", "web_ui", "file", "event", "database", "infra", "container", "composite"]},
              "artifact_ref": {"type": "string"},
              "adapter_family": {"type": "string"},
              "environment_class": {"type": "string", "enum": ["ephemeral", "bound_external", "live"]},
              "provisioning": {"type": "string"},
              "readiness": {"type": "array", "items": {"type": "object"}},
              "reset_policy": {"type": "string"},
              "evidence_policy": {"type": "string"}
            }
          }
        }
      }
    },
    "created_at": {"type": "string", "format": "date-time"}
  }
}`,
	"test-plan": `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "dft:test-plan",
  "title": "TestPlan",
  "description": "Verification contract defining BDD scenarios.",
  "type": "object",
  "required": ["increment_package_id", "scenarios"],
  "properties": {
    "increment_package_id": {"type": "string"},
    "requirement_ids": {"type": "array", "items": {"type": "string"}},
    "scenarios": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["id", "name", "then"],
        "properties": {
          "id": {"type": "string"},
          "name": {"type": "string"},
          "requirement_ids": {"type": "array", "items": {"type": "string"}},
          "description": {"type": "string"},
          "given": {"type": "array", "items": {"type": "string"}},
          "when": {"type": "array", "items": {"type": "string"}},
          "then": {"type": "array", "items": {"type": "string"}, "minItems": 1}
        }
      }
    }
  }
}`,
	"wbs": `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "dft:wbs",
  "title": "Work Breakdown Structure",
  "description": "Append-only work breakdown into independently executable specs.",
  "type": "object",
  "required": ["increment_package_id", "specs"],
  "properties": {
    "increment_package_id": {"type": "string"},
    "specs": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["id", "acceptance_criteria"],
        "properties": {
          "id": {"type": "string"},
          "description": {"type": "string"},
          "prompt_path": {"type": "string"},
          "acceptance_criteria": {"type": "array", "items": {"type": "string"}, "minItems": 1}
        }
      }
    }
  }
}`,
	"eval-surface-contract": `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "dft:eval-surface-contract",
  "title": "EvalSurfaceContract",
  "description": "Declares observable eval surfaces for an increment package.",
  "type": "object",
  "required": ["increment_package_id", "surfaces"],
  "properties": {
    "increment_package_id": {"type": "string"},
    "surfaces": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["id", "kind", "artifact_ref", "adapter_family", "environment_class"],
        "properties": {
          "id": {"type": "string"},
          "kind": {"type": "string", "enum": ["cli", "http_api", "graphql", "grpc", "web_ui", "file", "event", "database", "infra", "container", "composite"]},
          "artifact_ref": {"type": "string"},
          "adapter_family": {"type": "string"},
          "environment_class": {"type": "string", "enum": ["ephemeral", "bound_external", "live"]},
          "provisioning": {"type": "string"},
          "readiness": {"type": "array", "items": {"type": "object"}},
          "reset_policy": {"type": "string"},
          "evidence_policy": {"type": "string"}
        }
      }
    }
  }
}`,
	"lane-assignment": `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "dft:lane-assignment",
  "title": "LaneAssignment",
  "description": "Binds a spec to an executor lane.",
  "type": "object",
  "required": ["spec_id", "lane"],
  "properties": {
    "spec_id": {"type": "string"},
    "lane": {"type": "string"},
    "rationale": {"type": "string"}
  }
}`,
}

func runSchema(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "schema requires a type name. Available: %s\n", availableSchemaTypes())
		return 2
	}
	typeName := strings.TrimSpace(args[0])
	schema, ok := schemaJSON[typeName]
	if !ok {
		fmt.Fprintf(stderr, "unknown schema type %q. Available: %s\n", typeName, availableSchemaTypes())
		return 2
	}
	fmt.Fprintln(stdout, schema)
	return 0
}

func runValidate(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "validate requires a type name and a file path")
		fmt.Fprintln(stderr, "Usage: dft validate <type> <file>")
		return 2
	}
	typeName := strings.TrimSpace(args[0])
	filePath := strings.TrimSpace(args[1])

	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stderr, "read file %q: %v\n", filePath, err)
		return 2
	}

	switch typeName {
	case "increment-package":
		var dp domain.IncrementPackage
		if err := json.Unmarshal(data, &dp); err != nil {
			fmt.Fprintf(stderr, "parse increment-package: %v\n", err)
			return 2
		}
		if err := dp.Validate(); err != nil {
			fmt.Fprintf(stderr, "FAIL: %v\n", err)
			return 1
		}
	case "solution-design":
		var sd domain.SolutionDesign
		if err := json.Unmarshal(data, &sd); err != nil {
			fmt.Fprintf(stderr, "parse solution-design: %v\n", err)
			return 2
		}
		if err := sd.Validate(); err != nil {
			fmt.Fprintf(stderr, "FAIL: %v\n", err)
			return 1
		}
	case "wbs":
		var wbs domain.WBS
		if err := json.Unmarshal(data, &wbs); err != nil {
			fmt.Fprintf(stderr, "parse wbs: %v\n", err)
			return 2
		}
		if err := wbs.Validate(); err != nil {
			fmt.Fprintf(stderr, "FAIL: %v\n", err)
			return 1
		}
	case "eval-surface-contract":
		var esc domain.EvalSurfaceContract
		if err := json.Unmarshal(data, &esc); err != nil {
			fmt.Fprintf(stderr, "parse eval-surface-contract: %v\n", err)
			return 2
		}
		if err := esc.Validate(); err != nil {
			fmt.Fprintf(stderr, "FAIL: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "unknown validate type %q. Available: increment-package, solution-design, wbs, eval-surface-contract\n", typeName)
		return 2
	}

	fmt.Fprintln(stdout, "PASS")
	return 0
}

func availableSchemaTypes() string {
	types := make([]string, 0, len(schemaJSON))
	for k := range schemaJSON {
		types = append(types, k)
	}
	return strings.Join(types, ", ")
}
