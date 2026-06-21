## ADDED Requirements

### Requirement: Plan 9 release artifact promotion is validation-gated
Facts SHALL publish Plan 9 release artifacts only for Plan 9 tuples that have native validation.

#### Scenario: Plan 9 amd64 artifact eligibility
- **WHEN** `plan9/amd64` compile coverage and the Plan 9 native release gate are both passing
- **THEN** `plan9/amd64` MAY be added to the release artifact matrix

#### Scenario: Plan 9 artifact names
- **WHEN** Plan 9 release artifacts are produced
- **THEN** they MUST follow the existing artifact naming scheme `facts-<version>-plan9-<arch>`
- **AND** the embedded version MUST be reported by `facts --version`

#### Scenario: Unvalidated Plan 9 artifacts are not published
- **WHEN** the Go toolchain supports a Plan 9 architecture that lacks native validation
- **THEN** Facts MUST NOT publish an artifact for that tuple

### Requirement: Plan 9 acceptance verification
Facts SHALL run binary-level acceptance verification on Plan 9 before Plan 9 is documented as a release target.

#### Scenario: Plan 9 binary acceptance
- **WHEN** Plan 9 is promoted to a release target
- **THEN** acceptance verification MUST execute the real Plan 9 binary with representative CLI modes supported on Plan 9
- **AND** it MUST assert the Plan 9 release-gate fact set and exit codes against the live Plan 9 guest

#### Scenario: Plan 9 unsupported CLI behavior
- **WHEN** a representative CLI mode depends on an OS feature unavailable on Plan 9
- **THEN** the acceptance verification MUST document the omission and continue to validate the supported CLI modes

### Requirement: Plan 9 documentation matches promotion state
Facts SHALL distinguish lab-validated Plan 9 fact support from published release-target support until Plan 9 artifacts are actually shipped.

#### Scenario: Plan 9 supported facts before artifact promotion
- **WHEN** Plan 9 facts are implemented and native-gated but no Plan 9 artifact is published
- **THEN** documentation MUST describe Plan 9 as lab-validated fact support rather than a published release artifact target

#### Scenario: Plan 9 release target after artifact promotion
- **WHEN** Plan 9 artifacts are added to the release matrix
- **THEN** README and release documentation MUST list Plan 9 with only the validated architectures
