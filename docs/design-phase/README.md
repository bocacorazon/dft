# Design-Phase Docs

This directory contains documentation for the **upstream design phase** that
produces artifacts consumed by dft.

dft itself is documented as the **execution layer** elsewhere. The split is:

- **design phase**: decide what should be built and emit frozen artifacts
- **dft**: execute those artifacts and record deterministic run output

## What belongs here

Design-phase concerns include:

- refining a request into a usable spec or demand package
- building a WBS DAG
- choosing or authoring workflows
- selecting model families and model-tier mappings
- any front-end agent process that prepares the execution inputs

## Relationship to dft

dft does not require a single design toolchain. It only requires that the
resulting inputs exist and conform to the execution contracts.

See [submission-artifacts.md](submission-artifacts.md) for the artifact set dft
consumes regardless of provenance.
