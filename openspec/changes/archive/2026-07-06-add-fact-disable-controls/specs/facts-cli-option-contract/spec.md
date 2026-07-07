## ADDED Requirements

### Requirement: The --disable option is part of the shared option vocabulary

The `facts` CLI SHALL accept `--disable` as a valued, comma-separated, repeatable option in the shared option vocabulary, documented like any other non-hidden option, contributing fact and group names to the disabled set.

#### Scenario: --disable is accepted and documented

- **WHEN** `--disable packages,os` is parsed
- **THEN** validation MUST accept it as a valued option contributing `packages` and `os` to the disabled set
- **AND** `--disable` MUST appear in generated help and man output

#### Scenario: --disable composes with --no-block

- **WHEN** both `--disable packages` and `--no-block` are given
- **THEN** `--no-block` MUST clear the disabled set so nothing is disabled
