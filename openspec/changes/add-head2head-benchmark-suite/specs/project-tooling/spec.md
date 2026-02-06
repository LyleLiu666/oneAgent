## ADDED Requirements

### Requirement: The project MUST provide a repeatable head-to-head benchmark suite for end-to-end delivery quality
The project MUST provide a repeatable benchmark suite that runs a fixed task set and reports objective delivery-quality metrics for comparison across versions.

At minimum, the suite MUST report:
- first pass success rate
- recovery success rate
- human intervention count
- evidence completeness
- cost per successful delivery (best-effort)

#### Scenario: Benchmark run outputs a machine-readable report
- **GIVEN** a defined benchmark task set
- **WHEN** the benchmark suite runs
- **THEN** it outputs a machine-readable report containing all required metrics
- **AND** the report can be compared to a previous baseline run (best-effort)

### Requirement: Benchmark suite MUST support nightly and manual execution without blocking regular PR validation
The project MUST support scheduled and manual benchmark execution paths, and SHOULD avoid making benchmark runtime a mandatory blocker for regular PR CI.

#### Scenario: Nightly benchmark run produces trend artifacts
- **WHEN** the nightly benchmark workflow triggers
- **THEN** the suite runs against the configured task set
- **AND** publishes artifacts that include current values and baseline deltas (best-effort)

#### Scenario: Manual benchmark run is available for release decisions
- **WHEN** a maintainer triggers benchmark execution manually
- **THEN** the same benchmark suite runs with the same metric schema
