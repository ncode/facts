## MODIFIED Requirements

### Requirement: Host probes remain Session-injectable

Facts SHALL keep host I/O used for platform fact discovery reachable through the run-scoped Session seam so category behavior can be tested with injected native source data. The Session host seam SHALL be the only resolver host-I/O path: core fact resolvers MUST NOT read the host through raw `os`/`filepath`/`exec` calls or through parameter-injected reader/runner alternatives that duplicate the Session seam, and this MUST be structurally enforced by an automated check with a fixed, documented exclusion list (the seam implementation itself, the external-fact loader, the persistent cache, config parsing, syscall-tagged files, and test files). Category assemblies SHALL obtain platform identity through the Session (`s.goos()`) and environment values through the Session's host environment with Windows-only case-insensitive lookup, so platform-conditional assembly paths are exercisable with a fake host on any development platform. Pure parse functions keep their string and goos parameters per the category-split contract; recorded exceptions (process clock, `exec.LookPath` in the Linux distro probe, identity's uid/gid syscalls, `net.Interfaces`) remain injectable parameters where tests need them.

#### Scenario: Disk probes are injectable

- **WHEN** disk, partition, or mountpoint facts need command output, file reads, stat data, directory reads, glob matches, or platform identity
- **THEN** tests MUST be able to provide those inputs without reading the developer host directly

#### Scenario: Session command behavior is preserved

- **WHEN** a fact resolver executes a platform command through the Session host seam
- **THEN** command timeout, context cancellation, logging, and sanitized environment behavior MUST remain consistent with current Session command execution

#### Scenario: Resolver host I/O cannot bypass the seam

- **WHEN** a core fact resolver outside the documented exclusion list reads a file, lists a directory, stats a path, expands a glob, or executes a command through raw `os`/`filepath`/`exec` calls instead of the Session host seam
- **THEN** the automated seam check fails, identifying the offending file and call

#### Scenario: Category assembly is drivable onto another platform

- **WHEN** a test constructs a Session whose fake host reports a platform identity different from the test host (e.g. windows assembly driven from a Linux CI host)
- **THEN** the category assembly functions resolve using the fake host's platform identity, environment values, file contents, and command outputs, without reaching the real host

## ADDED Requirements

### Requirement: Host virtualization is gathered once per discovery

The host-virtualization signal gather (on Linux: DMI reads plus the dmidecode/virt-what/vmware/lspci command set; on Windows: the wmic/CIM and registry gather) SHALL run at most once per discovery, memoized on the run-scoped Session like other shared host probes, with the `virtual`/`is_virtual` facts, the `hypervisors` fact tree, and the uptime container gate all reading the same memoized gather input. Classification of the gathered input SHALL remain a pure derivation so memoizing the gather does not change any resolved fact value.

#### Scenario: Linux gather commands run once

- **WHEN** a discovery resolves `virtual`, `hypervisors`, and `system_uptime` on a Linux host
- **THEN** each virtualization gather command (dmidecode, virt-what, vmware, lspci) is executed at most once for that discovery, and all three consumers observe facts derived from the same gather

#### Scenario: Windows gather runs once

- **WHEN** a discovery resolves `virtual` and `hypervisors` on a Windows host
- **THEN** the wmic/CIM and registry virtualization gather executes at most once for that discovery

#### Scenario: Memoization is discovery-scoped

- **WHEN** the same Engine runs two discoveries
- **THEN** the second discovery re-gathers virtualization signals fresh (the memo lives on the per-discovery Session, not the Engine)

#### Scenario: Resolved values are unchanged by memoization

- **WHEN** the memoized gather input is classified for the `virtual` fact and for the `hypervisors` tree
- **THEN** each consumer's classification produces the same fact names and values as before memoization, including their documented divergences
