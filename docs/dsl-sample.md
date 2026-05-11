
```yaml
# speckit.yaml — Production speckit lifecycle flow

#

# Orchestrates the full specify → plan → tasks → implement → review → merge

# pipeline using the Copilot CLI as the execution engine.

#

# Usage:

#   iso run --flow flows/speckit.yaml \

#           --spec .pipeline/runtime/desc.md \

#           --work-dir /path/to/target-repo

#

# Requirements:

#   - `copilot` CLI in PATH (GitHub Copilot CLI)

#   - Target repo must have .github/agents/speckit.*.agent.md

#   - Target repo must have .specify/templates/

#

# Flow vars (can be overridden via CLI or env):

#   feature    — short feature description for prompts

#   spec-branch — set automatically by the specify stage (branch name)

 

flow:

  id: speckit

  version: "2.0"

  description: "Speckit lifecycle: specify → plan → tasks → implement → review → merge via Copilot CLI"

 

vars:

  feature: "Vite Project Scaffold - React+TypeScript frontend with Tailwind CSS"

  base_branch: "main"

  copilot_flags: "--allow-all -s --no-ask-user --autopilot"

 

defaults:

  timeout: "600s"

  max_retries: 1

 

stages:

  specify:

    timeout: "600s"

    steps:

      - name: run-specify

        type: tool

        timeout: "600s"

        capture: true

        run: |

          copilot -C "{{ work_dir }}" -p "Read the feature description from .pipeline/runtime/desc.md. Create a feature branch, create the spec directory and checklists directory under specs/<branch-name>/. Create the specification in specs/<branch-name>/spec.md following the spec template in .specify/templates/spec-template.md. Also create specs/<branch-name>/checklists/requirements.md using .specify/templates/checklist-template.md. The feature is: {{ feature }}. Output the branch name you created as the last line of your response, prefixed with BRANCH:" \

            --agent speckit.specify {{ copilot_flags }}

    post:

      - name: capture-branch

        type: tool

        capture: true

        export_as: spec-branch

        run: |

          branch=$(git rev-parse --abbrev-ref HEAD)

          echo "$branch"

    verify:

      checks:

        - fn: file_exists

          args: ["specs/{{ spec-branch }}/spec.md"]

        - fn: file_exists

          args: ["specs/{{ spec-branch }}/checklists/requirements.md"]

        - fn: os

          args: ["test $(wc -l < specs/{{ spec-branch }}/spec.md) -gt 50"]

 

  plan:

    timeout: "600s"

    steps:

      - name: run-plan

        type: tool

        timeout: "600s"

        run: |

          copilot -C "{{ work_dir }}" -p "Read the spec at specs/{{ spec-branch }}/spec.md. Create the technical plan following .specify/templates/plan-template.md. Write specs/{{ spec-branch }}/plan.md as the main plan. Also create: specs/{{ spec-branch }}/research.md (technology research), specs/{{ spec-branch }}/data-model.md (data structures and types), specs/{{ spec-branch }}/quickstart.md (getting started guide). If the feature has external interfaces, create specs/{{ spec-branch }}/contracts/cli.md. The feature is: {{ feature }}" \

            --agent speckit.plan {{ copilot_flags }}

    verify:

      checks:

        - fn: file_exists

          args: ["specs/{{ spec-branch }}/plan.md"]

        - fn: file_exists

          args: ["specs/{{ spec-branch }}/research.md"]

        - fn: file_exists

          args: ["specs/{{ spec-branch }}/data-model.md"]

        - fn: os

          args: ["test $(wc -l < specs/{{ spec-branch }}/plan.md) -gt 30"]

 

  tasks:

    timeout: "600s"

    steps:

      - name: run-tasks

        type: tool

        timeout: "600s"

        run: |

          copilot -C "{{ work_dir }}" -p "Read the spec at specs/{{ spec-branch }}/spec.md and the plan at specs/{{ spec-branch }}/plan.md. Generate an actionable, dependency-ordered task list following .specify/templates/tasks-template.md. Write the result to specs/{{ spec-branch }}/tasks.md. Each task should have a checkbox, clear description, and acceptance criteria." \

            --agent speckit.tasks {{ copilot_flags }}

    verify:

      checks:

        - fn: file_exists

          args: ["specs/{{ spec-branch }}/tasks.md"]

        - fn: os

          args: ["grep -q '\\- \\[' specs/{{ spec-branch }}/tasks.md"]

 

  implement:

    timeout: "1800s"

    steps:

      - name: run-implement

        type: tool

        timeout: "1800s"

        run: |

          copilot -C "{{ work_dir }}" -p "Read the tasks at specs/{{ spec-branch }}/tasks.md and the plan at specs/{{ spec-branch }}/plan.md. If .pipeline/runtime/verdict-failures.md exists, read it first and focus only on fixing the listed failing criteria and any directly related unfinished tasks before making other changes. Do not redo already-satisfied behavior or unrelated setup. Otherwise, implement all tasks in order. After completing each task, mark its checkbox as [x] in tasks.md. Ensure the code builds and tests pass." \

            --agent speckit.implement {{ copilot_flags }}

    post:

      - name: push-branch

        type: tool

        timeout: "60s"

        run: "git push origin {{ spec-branch }}"

    verify:

      checks:

        - fn: file_exists

          args: ["specs/{{ spec-branch }}/tasks.md"]

        - fn: os

          args: ["grep -c '\\- \\[x\\]' specs/{{ spec-branch }}/tasks.md | grep -v '^0$'"]

        - fn: os

          args: ["go build ./..."]

        - fn: os

          args: ["go test ./... -race -count=1"]

 

  review:

    timeout: "1800s"

    commit: false

    steps:

      - name: review-fix-loop

        type: loop

        max_iterations: 3

        do_until: true

        exit_when:

          no_critical_findings: true

        steps:

          - name: code-review

            type: tool

            timeout: "300s"

            capture: true

            run: |

              copilot -C "{{ work_dir }}" -p "Review all code changes on the current branch vs main. Output ONLY a JSON object with these fields: critical (int count of critical issues), important (int count of important issues), minor (int count of minor issues), findings (array of objects with severity, file, line, description, suggestion). Focus on bugs, security issues, and logic errors." \

                --agent dft.code-review {{ copilot_flags }} | tee specs/{{ spec-branch }}/review-findings.json

          - name: fix-findings

            type: tool

            timeout: "600s"

            run: |

              copilot -C "{{ work_dir }}" -p "Review the git diff of the current branch against main. Read the review findings at specs/{{ spec-branch }}/review-findings.json. Fix any critical or high severity code issues (bugs, security vulnerabilities, logic errors, missing error handling). If no issues need fixing, make no changes." \

                --agent speckit.implement {{ copilot_flags }}

    post:

      - name: commit-review-fixes

        type: tool

        timeout: "120s"

        run: |

          if [ -n "$(git status --porcelain)" ]; then

            copilot -C "{{ work_dir }}" -p "Look at the pending git changes and commit them with an appropriate message describing the review fixes" \

              --agent speckit.git.commit {{ copilot_flags }}

          fi

      - name: push-review

        type: tool

        timeout: "60s"

        run: "git push origin {{ spec-branch }}"

      - name: file-remaining-issues

        type: tool

        timeout: "300s"

        run: |

          findings_file="specs/{{ spec-branch }}/review-findings.json"

          if [ ! -f "$findings_file" ]; then

            echo "No review findings file, skipping issue creation"

            exit 0

          fi

          if command -v iso >/dev/null 2>&1; then

            iso gh-issues --findings "$findings_file" --spec-dir "{{ spec-branch }}" || echo "gh-issues failed (non-fatal), continuing"

          elif [ -x "./iso" ]; then

            ./iso gh-issues --findings "$findings_file" --spec-dir "{{ spec-branch }}" || echo "gh-issues failed (non-fatal), continuing"

          else

            echo "iso binary not found, skipping issue filing"

          fi

    verify:

      checks:

        - fn: os

          args: ["go build ./..."]

        - fn: os

          args: ["go test ./... -race -count=1"]

 

  merge:

    timeout: "600s"

    commit: false

    pre:

      - name: verify-tasks-complete

        type: tool

        timeout: "30s"

        run: |

          total=$(grep -E -c '^- \[( |x)\]' specs/{{ spec-branch }}/tasks.md || true)

          completed=$(grep -E -c '^- \[x\]' specs/{{ spec-branch }}/tasks.md || true)

          echo "Tasks: $completed/$total complete"

          test "$total" -gt 0 && test "$completed" -eq "$total"

      - name: verify-clean-tree

        type: tool

        timeout: "30s"

        run: |

          if [ -n "$(git status --porcelain)" ]; then

            echo "Working tree is dirty — committing remaining changes"

            git add -A && git commit -m "chore: clean working tree before merge"

            git push origin {{ spec-branch }}

          fi

      - name: merge-base-branch

        type: tool

        timeout: "120s"

        run: |

          git fetch origin {{ base_branch }}

          git merge origin/{{ base_branch }} --no-edit || {

            echo "Merge conflict detected — accepting ours for spec and runtime artifacts"

            git checkout --ours specs/ .pipeline/ desc.md 2>/dev/null || true

            git add .

            git commit --no-edit -m "merge: resolve conflicts with {{ base_branch }}"

          }

    steps:

      - name: run-tests

        type: tool

        timeout: "300s"

        run: "find . -maxdepth 1 -type d -name 'verdict-tests-*' -prune -exec rm -rf {} + && go test ./... -race -count=1"

      - name: create-pr

        type: tool

        timeout: "120s"

        capture: true

        export_as: pr_url

        run: |

          gh pr create \

            --title "{{ spec-branch }}: {{ feature }}" \

            --body "## Summary

 

          Automated speckit pipeline run for {{ spec-branch }}.

 

          ## Artifacts

          - specs/{{ spec-branch }}/spec.md

          - specs/{{ spec-branch }}/plan.md

          - specs/{{ spec-branch }}/research.md

          - specs/{{ spec-branch }}/data-model.md

          - specs/{{ spec-branch }}/tasks.md

 

          ## Pipeline

          All stages completed: specify → plan → tasks → implement → review → merge" \

            --base {{ base_branch }} 2>&1 | tail -1

      - name: wait-for-ci

        type: tool

        timeout: "300s"

        run: |

          pr_number=$(gh pr list --head {{ spec-branch }} --json number -q '.[0].number')

          if [ -z "$pr_number" ]; then

            echo "No PR found, skipping CI wait"

            exit 0

          fi

          checks=$(gh pr checks "$pr_number" --json name 2>/dev/null | grep -F -c 'name' || true)

          if [ "$checks" -eq 0 ]; then

            echo "No CI checks configured, skipping"

            exit 0

          fi

          gh pr checks "$pr_number" --watch --fail-fast || true

      - name: merge-and-retry

        type: loop

        max_iterations: 2

        do_until: true

        exit_when:

          exit_code: 0

        steps:

          - name: squash-merge

            type: tool

            timeout: "120s"

            capture: true

            run: |

              pr_number=$(gh pr list --head {{ spec-branch }} --json number -q '.[0].number')

              if [ -z "$pr_number" ]; then

                echo "No PR found to merge"

                exit 1

              fi

              gh pr merge "$pr_number" --squash --delete-branch 2>&1

          - name: fix-merge-issues

            type: tool

            timeout: "600s"

            run: |

              pr_number=$(gh pr list --head {{ spec-branch }} --json number -q '.[0].number')

              copilot -C "{{ work_dir }}" -p "The squash merge of PR #${pr_number} on branch {{ spec-branch }} failed. Diagnose the issue: check for merge conflicts, failing CI checks, or required reviews. Fix any code issues, resolve conflicts, ensure tests pass, then push the fixes. Do not attempt the merge itself." \

                --agent speckit.implement {{ copilot_flags }}

              git push origin {{ spec-branch }}

    verify:

      checks:

        - fn: os

          args: ["gh pr list --head {{ spec-branch }} --state merged --json number -q '.[0].number' | grep -q '[0-9]'"]
```