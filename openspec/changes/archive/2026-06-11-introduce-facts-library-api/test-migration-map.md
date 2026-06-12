# facter_test.go migration map (task 1.1)

Classification of every test in `facter_test.go` ahead of the Ruby-compat API removal.
Categories: **cli** (re-home as CLI contract test via `internal/app.Run`), **engine** (re-home as Engine-level test on the new `facts` API), **ruby-api** (dropped with the old surface; any embedded contract assertion noted).

Key facts that shaped the classification:

- `internal/app.Run` does NOT call `Resolve`/`ToUserOutput`; it has its own flag parser and drives internal helpers (`LoadExternalFactsWithBlocklist`, `LoadCustomFacts`, `CoreFactsWithRuby`, `LegacyFacts`, `SelectWithDottedFacts`, ...). So Resolve-only affordances (symbol-style `:fact` names via `normalizeQuery`, quoted-arg tokenizing of the single `args string`) are NOT CLI-observable and die with the Ruby API.
- The CLI deliberately diverges from `Resolve` on one validation: an empty-string `external-dir : ""` plus `no-external-facts` in config is ACCEPTED by the CLI (`TestRun_configAllowsNoExternalFactsWithEmptyExternalDir`) but rejected by `Resolve`. Do not port the Resolve-side strictness.
- `TestMain` in facter_test.go stubs `defaultExternalFactDirs` to nil so host `facts.d` dirs cannot leak into tests; the Engine test suite needs an equivalent seam.

## Summary
- 194 tests total: 54 cli, 77 engine, 63 ruby-api
- Of the cli tests: 39 already covered by internal/app/app_test.go; the rest covered by internal/app/contract_test.go (task 1.2). One test reclassified engine (TestToUserOutput_strictCustomNilFactIsResolved): 53 cli, 78 engine, 63 ruby-api. Three Resolve-vs-CLI divergences found and pinned (trailing options, strict nil custom-file fact, legacy blocklist vs explicit query) — see DIVERGENCE rows.

New CLI tests needed (consolidatable):
1. Legacy alias queries beyond uptime_hours: `processor0`, `hardwaremodel`, `id`/`gid` (one table-driven test) — from TestResolve_returnsLegacyProcessorAlias, TestResolve_specificHardwareModelLegacyFact, TestResolve_specificIdentityLegacyFacts.
2. Nil-fact rendering: queried resolved-nil custom fact prints JSON `null`; `--strict` treats resolved-nil as found (exit 0) — from TestToUserOutput_queriedCustomNilFactIsRenderedAsJSONNull, TestToUserOutput_strictCustomNilFactIsResolved.
3. Invalid array-index queries (`arr.3`, `arr.abc`, `arr.-1`) print empty output, exit 0 — from TestToUserOutput_indexesProgrammaticCustomArrayFact, TestResolve_digsIntoCustomFactArrays, TestToUserOutput_digsIntoExternalFactArrays.
4. `--force-dot-resolution` partial query of a dotted CUSTOM (.rb) fact prints legacy hash `{\n  c => "custom"\n}`; without the flag prints empty — from TestToUserOutput_dottedCustomFactPartialQueryRespectsForceDotResolution.
5. Options accepted after positional queries (`facter os.name --show-legacy uptime_hours`) — from TestResolve_acceptsOptionsAfterQueries.
6. Config-file dirs load facts when no CLI override: `global.external-dir` and `global.custom-dir` — from TestResolve_loadsExternalFactsFromConfig, TestResolve_loadsCustomFactsFromConfig.
7. Config `no-custom-facts : true` skips FACTERLIB; config `no-external-facts : true` and CLI `--no-external-facts` skip env/external facts — from TestResolve_configNoCustomFactsSkipsFacterlib, TestResolve_noExternalFactsSkipsEnvironmentExternalFacts, TestResolve_configNoExternalFactsSkipsEnvironmentExternalFacts.
8. Long-form unknown option (`--unknown-option`) is rejected with usage (short-flag variant already covered) — from TestResolve_rejectsUnknownOption.
9. Explicitly queried legacy fact bypasses `blocklist : [ "legacy" ]` — from TestResolve_configBlocklistLegacyKeepsExplicitLegacyQuery.

## Inventory

| Test | Class | Disposition / notes |
|---|---|---|
| TestToHash_includesStructuredCoreFacts | engine | Snapshot tree includes structured os fact with architecture/family/name keys |
| TestList_returnsSortedTopLevelFactNames | engine | Top-level fact name enumeration is sorted and matches snapshot-tree keys (List() dies; pin on Snapshot) |
| TestEach_yieldsFlattenedResolvedFactNames | engine | Flattened resolved view yields os.name, not top-level os (Each() dies; pin on Snapshot flat iteration) |
| TestEach_yieldsProgrammaticCustomNilFacts | engine | Programmatic custom fact resolving to nil appears in flattened iteration with nil value |
| TestToHash_omitsProgrammaticCustomNilFacts | engine | Nil-valued custom facts omitted from snapshot tree |
| TestFact_digsProgrammaticCustomStringMapLikeRubyHash | engine | Dotted query digs into map[string]string custom fact value |
| TestValueAndFactDowncaseUserQueryLikeRubyAPI | engine | Query lookup is case-insensitive (OS.NAME == os.name); Value and Fact agree |
| TestToUserOutput_formatsQueriedFact | cli | Single query default format prints bare value, exit 0; already covered by app_test.go: TestRun_queryLegacy |
| TestVersionString_returnsPublicFacterVersion | ruby-api | VersionString() alias dies; CLI version output covered by TestRun_version / TestRun_shortVersion |
| TestMeasurePrintsRubyStyleTimingWhenEnabled | ruby-api | Measure/SetTiming surface dies; embedded contract: timing line `fact 'x', took: (N.NNN) seconds` is CLI stdout contract, already pinned by TestRun_timingPrintsResolutionDuration |
| TestMeasureDoesNotPrintWhenTimingDisabled | ruby-api | Measure toggle behavior; callback-still-runs nuance dies with API |
| TestWarnOnce_emitsOnceBeforeClearMessages | ruby-api | WarnOnce/OnMessage surface dies; embedded contract: once-only dedupe + `WARN Facter - <msg>` text, preserved by engine test TestValue_warnsOnceWhenNoCustomFactsLoadFromSearchPath... and CLI TestRun_externalExecutableStderrIsWrittenAsWarning |
| TestRubyStyleOnceAliasesDelegateToPublicMessageMethods | ruby-api | Debugonce/Warnonce aliases die; once-only semantics preserved as above |
| TestLogExceptionUsesErrorBacktraceWhenTraceIsEnabled | ruby-api | LogException + Backtrace() interface dies with API |
| TestLogExceptionOmitsBacktraceWhenTraceEnabledButErrorBacktraceIsEmptyLikeRuby | ruby-api | LogException empty-backtrace nuance dies |
| TestLookup_aliasesFact | ruby-api | Lookup alias for Fact dies; Fact lookup contract pinned by engine tests |
| TestAdd_resolvesProgrammaticCustomFactLazily | engine | Programmatic fact resolver runs lazily, exactly once, value cached within run |
| TestDefineFact_aliasesAddForProgrammaticCustomFacts | ruby-api | DefineFact alias dies; programmatic registration pinned by TestAdd_registersProgrammaticCustomFact (engine) |
| TestValue_missingFactDoesNotResolveUnrelatedProgrammaticCustomFacts | engine | Querying a missing fact must not execute unrelated registered resolvers |
| TestAdd_allowsNestedValueCallsFromProgrammaticCustomFact | engine | Resolver may query another fact (nested lookup) without deadlock |
| TestValue_digsProgrammaticCustomMapWithStringifiedKeys | engine | map[any]any keys stringified for dotted digging |
| TestValue_rejectsQueryWithNullByte | engine | NUL byte in query rejected (Ruby API panics; new API should return error - decide signature, keep rejection) |
| TestLoadfacts_resolvesFactsForRubyCompatibility | ruby-api | Loadfacts() alias dies |
| TestClear_resetsOptionStateToDefaults | ruby-api | Clear() global option-state lifecycle dies (HTTPDebug/Trace/Timing/Sequential/ForceDotResolution) |
| TestWhich_returnsAbsoluteExecutableFromSearchPath | ruby-api | Which() executor utility dies; expansion still backs custom-fact `execute` internally - move tests with internal executor if extracted |
| TestWhich_returnsAbsolutePathOnlyWhenExecutableFile | ruby-api | Which() execute-bit/dir semantics; same note as above |
| TestWhich_doesNotSearchWorkingDirectoryWhenPathIsEmpty | ruby-api | Which() does not fall back to CWD; same note |
| TestExecutableSearchPaths_posixAppendsRubyDefaultPaths | ruby-api | executableSearchPaths appends /sbin:/usr/sbin (Ruby parity); internal helper, move if executor extracted |
| TestExecutableSearchPaths_windowsSplitsPathOnSemicolonsLikeRubyExecutor | ruby-api | Windows PATH splitting; internal helper |
| TestWhichInPath_windowsUsesPATHEXTWhenExecutableHasExtension | ruby-api | Windows PATHEXT handling; internal helper |
| TestWhichInPath_windowsUsesDefaultExtensionsWhenPATHEXTIsMissing | ruby-api | Windows default extension fallback; internal helper |
| TestWhichInPathEnv_windowsHonorsExplicitEmptyPATHEXTLikeRubyExecutor | ruby-api | Explicit empty PATHEXT nuance; internal helper |
| TestWhichInPathEnv_windowsAllowsEchoBuiltinLikeRubyExecutor | ruby-api | echo builtin allowance; internal helper |
| TestWhichInPathEnv_windowsChecksAbsolutePathsLikeRubyExecutor | ruby-api | Windows drive/UNC absolute path checks; internal helper |
| TestExpandCommand_expandsBinaryAndPreservesArguments | ruby-api | ExpandCommand() dies as public API; expansion semantics back custom-fact execute - keep as internal executor tests |
| TestExpandCommand_quotesExpandedBinaryWithSpaces | ruby-api | Quoting of expanded binary with spaces; internal executor test |
| TestExpandCommand_expandsQuotedBinary | ruby-api | Already-quoted binary expansion; internal executor test |
| TestExpandCommand_returnsEmptyWhenBinaryIsMissing | ruby-api | Missing binary returns empty; internal executor test |
| TestToUserOutput_formatsJSONQuery | cli | --json single query renders JSON map with queried key; already covered by app_test.go: TestRun_queryJSON |
| TestToUserOutput_noRubyOmitsRubyFactsFromFullOutput | cli | --no-ruby omits ruby facts from full JSON; already covered by app_test.go: TestRun_noRubyOmitsRubyFacts |
| TestResolve_noRubyOmitsRubyFactsFromFullOutput | cli | Same contract via Resolve; already covered by app_test.go: TestRun_noRubyOmitsRubyFacts |
| TestResolve_configNoRubyOmitsRubyFactsFromFullOutput | cli | Config global.no-ruby; already covered by app_test.go: TestRun_configNoRubyOmitsRubyFacts |
| TestResolve_configShowLegacyIncludesLegacyFactsFromFullOutput | cli | Config global.show-legacy; already covered by app_test.go: TestRun_configShowLegacyPrintsLegacyFacts |
| TestResolve_acceptsConcatenatedShortCompatibilityFlags | cli | -jd concatenated short flags; already covered by app_test.go: TestRun_concatenatedShortJSONAndDebugFlags (also _ShortTimingAndDebug, _ShortPuppetJSONDebugAndTiming) |
| TestResolve_queryCompatibilityFlagsUpdatePublicOptionState | cli | --trace/--http-debug/--sequential accepted; already covered by app_test.go: TestRun_queryCompatibilityFlagsUpdatePublicOptionState. Caveat: both assert dying option-state getters; the surviving CLI contract is flag acceptance - the app test must be reworded when getters go |
| TestResolve_acceptsShortOptionWithEquals | cli | -l=debug short-with-equals; already covered by app_test.go: TestRun_logLevelDebugOutputsDebugLogs (short variant) + TestRun_listCacheGroupsAcceptsShortConfigEquals (-c= form) |
| TestResolve_returnsLegacyProcessorAlias | cli | processor0 dynamic legacy alias query resolves; covered by contract_test.go: TestRun_specificLegacyAliasQueriesResolve |
| TestToUserOutput_strictMissingFactReturnsNonZeroStatus | cli | --strict + missing fact: exit 1, missing key rendered null, resolved key kept; already covered by app_test.go: TestRun_strictLogsMissingFactErrorWhenQueriedFactIsMissing (also pins stderr `ERROR Facter - fact "missing_fact" does not exist.`) |
| TestToUserOutput_strictCustomNilFactIsResolved | engine | Programmatic nil-resolved fact counts as found under strict — API-level semantics; pin via Engine `WithFact` nil resolver (3.7). DIVERGENCE: at the CLI a custom-fact FILE resolving to nil is reported missing under --strict (exit 1) — pinned by contract_test.go: TestRun_strictQueriedCustomNilFactFromFileIsMissing |
| TestToUserOutput_queriedCustomNilFactIsRenderedAsJSONNull | cli | Queried nil custom fact renders JSON null, exit 0; covered by contract_test.go: TestRun_queriedCustomNilFactRendersJSONNull |
| TestToHash_omitsCustomNilFacts | engine | Duplicate of TestToHash_omitsProgrammaticCustomNilFacts; fold into one Engine test |
| TestAdd_normalizesTimeCustomFactToISO8601 | engine | time.Time custom value normalized to ISO8601 string (2020-01-01T00:00:00Z) |
| TestValue_indexesProgrammaticCustomArrayFact | engine | Array fact index digging (arr.0 == first); out-of-range returns nil |
| TestToUserOutput_indexesProgrammaticCustomArrayFact | cli | Valid index prints element; invalid indexes (arr.3/arr.abc/arr.-1) print empty, exit 0; valid case covered by TestRun_queryCustomDirRubyArrayFact, invalid-index case by contract_test.go: TestRun_invalidCustomArrayIndexQueriesPrintNothing |
| TestValue_digsIntoTypedProgrammaticCustomMapFact | engine | map[string]string custom fact dotted digging |
| TestFact_indexesProgrammaticCustomArrayFact | engine | Fact() array index dig; out-of-range returns not-found |
| TestValueAndFactRejectInvalidProgrammaticCustomArrayIndexes | engine | arr.3 / arr.abc / arr.-1 all resolve to nil/not-found (no nearest-match fallback) |
| TestToUserOutput_dottedCustomFactPartialQueryRespectsForceDotResolution | cli | Partial query of dotted custom fact: empty by default; with force-dot prints legacy hash `{ c => "custom" }`; covered by contract_test.go: TestRun_forceDotResolutionAllowsPartialDottedCustomFactQuery |
| TestToUserOutput_dottedExternalFactPartialQueryRespectsForceDotResolution | cli | External-fact variant; already covered by app_test.go: TestRun_forceDotResolutionAllowsPartialDottedExternalFactQuery (+ config variant); note: default-off empty-output half should be kept |
| TestToHash_includesIPv6Aliases | engine | Snapshot has ipaddress6 root alias identical to networking.ip6 |
| TestToHash_includesLegacyNetworkAliases | engine | Snapshot includes fqdn/hostname/ipaddress root aliases |
| TestToHash_includesStandardCoreRootFacts | engine | Snapshot includes dmi, identity, is_virtual, kernel*, ruby, system_uptime, timezone, virtual |
| TestToHash_includesProcessorExtensions | engine | processors.extensions is sorted non-empty []string |
| TestValue_digsIntoStructuredFactsAndArrays | engine | os.name, processors.models.0 dig; negative index nil |
| TestValue_missingNestedQueryDoesNotReturnNearestFact | engine | Over-deep/invalid nested queries return nil, never the nearest ancestor value |
| TestValue_acceptsRubySymbolStyleFactNames | ruby-api | `:facterversion` symbol prefix stripping is Resolve/Value-only (normalizeQuery); CLI findFact only downcases - no surviving contract |
| TestFact_canonicalizesMixedCaseQueries | engine | Mixed-case query canonicalized; Fact.Name reports canonical os.name |
| TestAdd_registersProgrammaticCustomFact | engine | Programmatic fact registration resolvable via Value and Fact |
| TestValue_reusesResolvedProgrammaticCustomFact | engine | Resolver called once; falsey (false) value still cached; Fact shares cache |
| TestFlushInvalidatesProgrammaticCustomFactValues | ruby-api | Flush() lifecycle dies; within-run caching pinned by TestValue_reusesResolvedProgrammaticCustomFact (engine); cross-snapshot refresh is the new Engine/Snapshot model |
| TestFlushInvalidatesCoreFacts | ruby-api | Flush() invalidates core fact cache; replaced by new-Snapshot-per-collect semantics |
| TestFlushDoesNotReloadRegisteredCustomFactFilesLikeRubyAPI | ruby-api | Flush-vs-file-reload distinction dies with Flush |
| TestResetAllowsRegisteredCustomFactFilesToReloadLikeRubyAPI | ruby-api | Reset() reload semantics die; new Engine construction defines reload boundaries |
| TestResetClearsSearchPathsButPreservesProgrammaticCustomFacts | ruby-api | Reset() partial-state lifecycle dies |
| TestSearchPreservesRelativeCustomFactDirectories | ruby-api | Search()/SearchPath() echo of relative dirs dies; Engine option ordering covered by engine custom-dir tests |
| TestResetPreservesExternalFactLoadingToggle | ruby-api | Reset()+LoadExternal interplay dies |
| TestResetInvalidatesCachedCoreFacts | ruby-api | Reset() cache invalidation dies; per-Snapshot freshness replaces it |
| TestAdd_rejectsProgrammaticCustomFactWithNullBytes | engine | NUL in fact name rejects registration with warning; NUL in value detected at resolution (nil + warning); warning text contains "null byte" |
| TestDefineFact_zeroWeightCustomFactDefersToCoreFactByName | ruby-api | Zero-weight programmatic facts deferring to core was Ruby resolution-merge emulation; dropped with the Ruby API. The new WithFact overrides core (pinned by engine_test.go: TestWithFact_overridesCoreFactAndLosesToExternal) |
| TestDefineFact_zeroWeightCustomRootFactDefersToCoreFactByExactName | ruby-api | Zero-weight programmatic facts deferring to core was Ruby resolution-merge emulation; dropped with the Ruby API. The new WithFact overrides core (pinned by engine_test.go: TestWithFact_overridesCoreFactAndLosesToExternal) |
| TestLoadCustomFacts_positiveWeightCustomFactOverridesCoreFactByName | engine | DSL has_weight 1 overrides core fact (custom precedence via weight) |
| TestValue_coreFactDoesNotLoadUnrelatedCustomFactFilesLikeRubyFactManager | engine | Core fact query does not execute unrelated custom .rb files (no side-effect diagnostics) |
| TestValue_warnsOnceWhenNoCustomFactsLoadFromSearchPathLikeRubyCollection | engine | Empty custom dir warns exactly once: `WARN Facter - No facts loaded from <dir>` (once-only diagnostics contract) |
| TestDefineFact_legacyPatternNameResolvesCustomFact | engine | Custom fact whose name matches a legacy alias pattern (network_nexthop_ip) still resolves |
| TestValue_externalFactOverridesCoreRootFactByExactName | engine | Precedence: external file fact named os overrides core root os |
| TestForceDotResolutionMergesDottedCustomFactIntoRootFact | engine | Force-dot merges site.role custom fact into structured site root; off by default |
| TestValue_dottedCustomFactPartialQueriesRespectForceDotResolution | engine | a.b.c custom: partial a.b / a queries nil by default, merged maps with force-dot; over-deep a.b.c.d always nil |
| TestFact_dottedCustomFactPartialQueriesRespectForceDotResolution | engine | Same via Fact(): not-found by default, structured values with force-dot |
| TestClear_removesProgrammaticCustomFacts | ruby-api | Clear() lifecycle dies; Engine instances replace global state |
| TestEach_iteratesResolvedFactsInSortedOrder | engine | Flattened iteration sorted; includes core (facterversion) and custom facts |
| TestLoadFacts_resolvesWithoutChangingFacts | ruby-api | LoadFacts() alias dies |
| TestValue_returnsNetworkingInterfaces | engine | networking.interfaces resolves to non-empty map |
| TestDebugHonorsDebuggingToggle | ruby-api | Debugging()/SetDebugging/OnMessage die; `DEBUG Facter - <msg>` text format pinned at CLI by TestRun_logLevelDebugOutputsDebugLogs / TestRun_configCliDebugEmitsDebugLogs |
| TestWarnAndOnceMessages | ruby-api | Warn/WarnOnce/DebugOnce surface dies; once-only + `WARN Facter -` format preserved via engine diagnostics tests and CLI stderr tests |
| TestOnceMessagesWorkWithoutExplicitClearMessages | ruby-api | Internal messageState lazy-init nuance dies |
| TestInfoAndErrorMessages | ruby-api | Info/Error surface dies; `INFO Facter -` / `ERROR Facter -` formats pinned at CLI by TestRun_verboseOutputsInfoLogs and TestRun_strictLogsMissingFactErrorWhenQueriedFactIsMissing |
| TestMessagesUseRubyStyleStringification | ruby-api | Ruby inspect-style stringification of nil/arrays/hashes in messages dies (no CLI path emits non-string message values) |
| TestOnMessageWithLevelReceivesLevelAndUnformattedMessage | ruby-api | OnMessageWithLevel dies; if new Engine exposes structured diagnostics (level, message), re-pin there |
| TestShowTimeHonorsTimingToggle | ruby-api | ShowTime + ANSI-green stderr line dies; CLI --timing output pinned by TestRun_timingPrintsResolutionDuration |
| TestLoadExternalLogsDebugMessage | ruby-api | LoadExternal debug text references Facter.load_external - Ruby-specific, dies |
| TestLoadExternalEnabledReportsCurrentState | ruby-api | LoadExternalEnabled getter dies |
| TestClearClearsOnceMessages | ruby-api | Clear() resets once-dedupe sets; dies with Clear; per-Engine dedupe is the new model |
| TestLogException_logsMessageOrError | ruby-api | LogException message/error/nil fallback formatting dies |
| TestLogException_includesStackWhenTraceEnabled | ruby-api | LogException Go-stack-on-trace dies |
| TestLogException_traceDoesNotAppendStackForNonErrorInput | ruby-api | LogException non-error input nuance dies |
| TestPublicOptionToggles | ruby-api | HTTPDebug/Trace/Sequential/ForceDotResolution toggle getters/setters die; force-dot survives as Engine option (engine tests) and CLI flag (app tests) |
| TestCoreValue_returnsOnlyCoreFactValues | ruby-api | CoreValue() dies; core-vs-override precedence pinned by engine precedence tests (external overrides core etc.) |
| TestCoreValue_returnsNilForMissingFact | ruby-api | CoreValue missing-fact nil dies with API |
| TestToHash_includesPrimaryNetworkingInterface | engine | networking.primary names an entry in networking.interfaces (skips when no primary IPv4) |
| TestValue_loadsDefaultExternalFactDirectory | engine | Default external fact dirs (facts.d) loaded when none registered (uses defaultExternalFactDirs seam) |
| TestValue_registeredExternalFactDirectoryOverridesDefaultExternalFactDirectory | engine | Registering external dirs replaces the defaults entirely |
| TestResolve_loadsDefaultExternalFactDirectory | cli | Default facts.d honored by CLI; already covered by app_test.go: TestRun_queryDefaultExternalFactDirectory |
| TestValue_loadsRegisteredExternalFacts | engine | Registered external dir .txt fact resolves |
| TestValue_reusesResolvedExternalFactLikeRubyAPI | engine | External fact value cached within run; file edits invisible until new snapshot |
| TestFact_sharesValueLookupCacheLikeRubyAPI | engine | Value and Fact share one resolution cache (snapshot consistency) |
| TestValue_dottedExternalFactPartialQueriesRespectForceDotResolution | engine | a.b.c external fact: partial queries nil by default, merged maps with force-dot |
| TestFact_dottedExternalFactPartialQueriesRespectForceDotResolution | engine | Same via Fact() |
| TestValue_warnsWhenExternalFactResolutionIsRecursive | engine | FACTER_EXTERNAL_FACTS_RUNNING guard: executable external facts skipped + "Recursion detected" warning (CLI side covered by TestRun_warnsAndSkipsExecutableExternalFactsDuringRecursiveResolution) |
| TestValue_warnsWhenExternalExecutableWritesStderr | engine | Executable stderr surfaces as warning `Command <path> completed with the following stderr message: ...`; value still parsed (CLI text pinned by TestRun_externalExecutableStderrIsWrittenAsWarning) |
| TestValue_warnsWhenExternalFactValueContainsNullByte | engine | External YAML value with NUL dropped; warning contains "contains a null byte reference" |
| TestResolve_warnsWhenExternalFactResolutionIsRecursive | cli | Recursion guard at CLI; already covered by app_test.go: TestRun_warnsAndSkipsExecutableExternalFactsDuringRecursiveResolution |
| TestResolve_configForceDotResolutionAllowsPartialDottedExternalFactQuery | cli | Config global.force-dot-resolution; already covered by app_test.go: TestRun_configForceDotResolutionAllowsPartialDottedExternalFactQuery |
| TestResolve_acceptsShortConfigOptionWithEquals | cli | -c=<path>; already covered by app_test.go: TestRun_listCacheGroupsAcceptsShortConfigEquals |
| TestLoadExternalControlsExternalFactLoading | ruby-api | LoadExternal toggle + Flush/Reset interplay dies; disable-external survives as Engine option and CLI --no-external-facts (see needs-new item 7) |
| TestToUserOutputHonorsLoadExternalToggle | ruby-api | LoadExternal affecting ToUserOutput dies; CLI equivalent is --no-external-facts (needs-new item 7) |
| TestValue_loadsEnvironmentExternalFacts | engine | FACTER_<name> env var resolves as external fact |
| TestPuppetFactsRegistersPuppetVersionFact | ruby-api | PuppetFacts() public API dies; CLI --puppet covered by TestRun_puppetLoadsPuppetVersionFact (internal facter.PuppetFacts helper remains app-side) |
| TestToHash_includesRegisteredCustomAndExternalFacts | engine | Snapshot tree includes registered custom (.rb) and external (.txt) facts |
| TestValue_externalFactOverridesCustomFactByName | engine | Precedence: external > custom for same name |
| TestFact_returnsResolvedFactForCoreQuery | engine | Fact() for dotted core query returns Name == query and non-nil Value |
| TestResolvedFactString_returnsFactValue | ruby-api | ResolvedFact.String() dies with type |
| TestResolvedFactString_returnsEmptyStringForNilValue | ruby-api | ResolvedFact.String() nil case dies |
| TestFact_returnsResolvedFactForCustomNilValue | engine | Resolved-nil custom fact is found-with-nil, distinct from missing (new API: found=true, value=nil) |
| TestFact_returnsNilForMissingCoreQuery | engine | Over-deep/missing queries return not-found |
| TestValue_returnsLoadAverageIntervals | engine | load_averages.1m/5m/15m resolve on darwin/linux |
| TestValue_returnsMacOSDMIProductName | engine | darwin dmi.product.name resolves; embedded legacy `productname` --show-legacy assertion should ride along in the darwin engine/CLI legacy coverage |
| TestMethod_returnsFactValueForMethodStyleLookup | ruby-api | Method() dies |
| TestRespondTo_reportsMethodStyleFactAvailability | ruby-api | RespondTo() dies |
| TestResolve_returnsQueriedFacts | cli | Multi-query invocation returns each queried key; already covered by app_test.go: TestRun_strictLogsMissingFactErrorWhenQueriedFactIsMissing (two queries) + TestRun_queryJSON |
| TestResolve_acceptsOptionsAfterQueries | cli | DIVERGENCE: Resolve(string) permuted trailing options; the CLI does not — flag parsing stops at the first query and trailing option-like tokens become queries. Current CLI behavior pinned by contract_test.go: TestRun_trailingOptionTokensAreTreatedAsQueries |
| TestResolve_withoutQueryReturnsStructuredFactHash | cli | No query: full structured map, no flat os.name children; already covered by app_test.go: TestRun_noQueryJSONReturnsFullFactMap (+ TestRun_noQueryPrintsKnownCoreFacts for legacy format) |
| TestResolve_acceptsRubySymbolStyleFactNames | ruby-api | `:facterversion` normalization lives only in Resolve's parser (normalizeQuery); app.Run does not strip `:` - no CLI contract |
| TestResolve_showLegacyIncludesLegacyFacts | cli | --show-legacy adds uptime* legacy facts; already covered by app_test.go: TestRun_showLegacyPrintsLegacyFacts |
| TestResolve_noShowLegacySuppressesLegacyFacts | cli | --no-show-legacy; already covered by app_test.go: TestRun_noShowLegacySuppressesLegacyFacts |
| TestResolve_specificLegacyFact | cli | Explicit legacy fact query (uptime_hours); already covered by app_test.go: TestRun_querySpecificLegacyFact |
| TestResolve_specificHardwareModelLegacyFact | cli | hardwaremodel legacy alias; covered by contract_test.go: TestRun_specificLegacyAliasQueriesResolve |
| TestResolve_specificIdentityLegacyFacts | cli | id/gid legacy aliases mirror identity.user/group; covered by contract_test.go: TestRun_specificIdentityLegacyFactQueriesMirrorIdentityFact |
| TestResolve_loadsExternalFactsFromDir | cli | --external-dir .txt fact; already covered by app_test.go: TestRun_queryExternalTxtFact |
| TestResolve_acceptsQuotedExternalDirWithSpaces | ruby-api | Quoted-token splitting is an artifact of Resolve(args string); real CLI receives argv from the shell - no surviving contract (app.Run with a spaced dir as one argv element already works) |
| TestResolve_normalizesExternalFactNamesFromDir | engine | External fact names downcased (Site_Location -> site_location) - input normalization contract |
| TestResolve_loadsExternalFactsFromConfig | cli | Config global.external-dir loads facts; covered by contract_test.go: TestRun_configExternalDirLoadsExternalFacts |
| TestResolve_cliExternalDirOverridesConfiguredExternalDir | cli | CLI --external-dir replaces configured dirs; already covered by app_test.go: TestRun_cliExternalDirOverridesConfiguredExternalDir |
| TestResolve_loadsCustomFactsFromDir | cli | --custom-dir .rb fact; already covered by app_test.go: TestRun_queryCustomDirRubyFact |
| TestResolve_loadsCustomFactsFromConfig | cli | Config global.custom-dir loads facts; covered by contract_test.go: TestRun_configCustomDirLoadsCustomFacts |
| TestResolve_cliCustomDirOverridesConfiguredCustomDir | cli | CLI --custom-dir replaces configured dirs; already covered by app_test.go: TestRun_cliCustomDirOverridesConfiguredCustomDir |
| TestResolve_loadsCustomFactsWithStringNamesFromDir | engine | DSL: Facter.add('string_name') accepted |
| TestResolve_skipsCustomFactsWithNonMatchingKernelConfine | engine | DSL: confine kernel: 'x' skips fact when kernel differs |
| TestResolve_loadsCustomFactsWithBraceSetcode | engine | DSL: setcode { 'web' } brace form |
| TestResolve_loadsCustomFactsWithExecutionSetcode | engine | DSL: setcode { Facter::Core::Execution.execute('cmd') } runs command |
| TestResolve_standaloneFacterValueDoesNotOverridePreviousCustomFact | engine | DSL: bare Facter.value(:kernel) between adds must not clobber earlier fact definitions |
| TestResolve_loadsCustomFactsWithHashValues | engine | DSL: hash literal values; dotted dig into role and tags.1 |
| TestResolve_loadsCustomFactsWithIntegerAndNilValues | engine | DSL: integer 7 stays int; setcode { nil } resolves to nil |
| TestResolve_loadsCustomFactsWithBooleanValues | engine | DSL: true/false values preserved as booleans |
| TestResolve_loadsCustomFactsFromFacterlib | cli | FACTERLIB env dir honored at CLI; already covered by app_test.go: TestRun_loadsCustomFactsFromFacterlib |
| TestValue_loadsCustomFactsFromFacterlib | engine | FACTERLIB env dir honored by library resolution (Engine input contract) |
| TestResolve_noCustomFactsSkipsFacterlib | cli | --no-custom-facts skips FACTERLIB; already covered by app_test.go: TestRun_noCustomFactsSkipsFacterlib |
| TestResolve_configNoCustomFactsSkipsFacterlib | cli | Config global.no-custom-facts skips FACTERLIB; covered by contract_test.go: TestRun_configNoCustomFactsSkipsFacterlib |
| TestResolve_noRubySkipsCustomFacts | cli | --no-ruby skips custom facts; already covered by app_test.go: TestRun_noRubySkipsCustomFactLoading |
| TestResolve_digsIntoCustomFactArrays | cli | Custom array fact index queries incl. invalid; valid case covered by app_test.go: TestRun_queryCustomDirRubyArrayFact (invalid-index gap tracked at TestToUserOutput_indexesProgrammaticCustomArrayFact) |
| TestResolve_loadsExternalFactsFromEnvironment | cli | FACTER_x env fact at CLI; already covered by app_test.go: TestRun_queryExternalEnvironmentFact |
| TestResolve_loadsEnvironmentExternalFactsWithoutUnderscore | engine | FACTERname (no underscore) prefix also accepted - env fact name parsing contract |
| TestResolve_environmentExternalFactsOverrideCoreFacts | engine | Precedence: env external > core for queried fact (CLI side: TestRun_facterversionQueryAllowsExternalOverride) |
| TestResolve_environmentExternalFactsOverrideCoreFactsWithoutQuery | engine | Same precedence in full (no-query) output (CLI side: TestRun_externalFactOverridesCoreFactInFullJSON) |
| TestResolve_environmentExternalFactsOverrideExternalFactFiles | engine | Precedence: env external > file external |
| TestResolve_noExternalFactsSkipsEnvironmentExternalFacts | cli | --no-external-facts skips env facts; covered by contract_test.go: TestRun_noExternalFactsSkipsEnvironmentExternalFacts |
| TestResolve_configNoExternalFactsSkipsEnvironmentExternalFacts | cli | Config global.no-external-facts skips env facts; covered by contract_test.go: TestRun_configNoExternalFactsSkipsEnvironmentExternalFacts |
| TestResolve_digsIntoExternalFactArrays | cli | External YAML array index; already covered by app_test.go: TestRun_queryExternalYAMLArrayIndex (invalid-index gap tracked above) |
| TestFact_digsIntoExternalFactArrays | engine | Fact() digs external YAML arrays; invalid indexes not-found |
| TestToUserOutput_digsIntoExternalFactArrays | cli | Same at output layer; already covered by app_test.go: TestRun_queryExternalYAMLArrayIndex (invalid-index gap tracked above) |
| TestResolve_rejectsNoExternalFactsWithExternalDir | cli | Conflict --external-dir + --no-external-facts; already covered by app_test.go: TestRun_rejectsNoExternalFactsWithExternalDir |
| TestResolve_rejectsConflictingPuppetCompatibilityOptions | cli | --puppet vs --no-puppet/--no-ruby/--no-custom-facts conflicts with exact error text; already covered by app_test.go: TestRun_rejectsConflictingPuppetCompatibilityOptions |
| TestResolve_rejectsUnknownOption | cli | Unknown long option rejected; short-flag variant covered by app_test.go: TestRun_concatenatedShortFlagsRejectUnknownOption; long form covered by contract_test.go: TestRun_rejectsUnknownLongOption |
| TestResolve_rejectsNoExternalFactsWithExternalDirEquals | cli | Conflict with --external-dir=<dir> form; already covered by app_test.go: TestRun_rejectsNoExternalFactsWithExternalDirEquals |
| TestResolve_configRejectsNoExternalFactsWithExternalDir | cli | Config conflict rejection; already covered by app_test.go: TestRun_rejectsInvalidOptionPairsFromConfig. WARNING: this Resolve test uses empty `external-dir : ""` which the CLI intentionally ALLOWS (TestRun_configAllowsNoExternalFactsWithEmptyExternalDir) - do not port the empty-dir rejection |
| TestResolve_configBlocklistLegacySuppressesLegacyFacts | cli | blocklist [legacy] hides legacy from --show-legacy; already covered by app_test.go: TestRun_configBlocklistLegacySuppressesLegacyFacts |
| TestResolve_configBlocklistLegacyKeepsExplicitLegacyQuery | cli | DIVERGENCE: Resolve kept explicitly queried legacy facts despite a legacy blocklist; the CLI suppresses them. Current CLI behavior pinned by contract_test.go: TestRun_configBlocklistLegacySuppressesExplicitLegacyQuery |
| TestResolve_noBlockIgnoresConfiguredBlocklist | cli | --no-block disables blocklist; already covered by app_test.go: TestRun_noBlockIgnoresConfiguredBlocklist |
| TestResolve_configBlocklistGroupSuppressesGroupFacts | cli | blocklist [networking] hides group incl. hostname alias; already covered by app_test.go: TestRun_configBlocklistGroupSuppressesGroupFacts |
| TestResolve_configBlocklistCustomFactGroupSuppressesGroupFacts | cli | fact-groups-defined group in blocklist; already covered by app_test.go: TestRun_configBlocklistExpandsConfiguredFactGroup |
| TestSearchExternal_registersExternalFactDirectory | engine | External dir registration is reflected in lookup (path-echo getter dies; registration->resolution contract survives as Engine option) |
| TestValue_logsExternalFactNullByteValueAndReturnsNil | engine | Near-duplicate of TestValue_warnsWhenExternalFactValueContainsNullByte (NUL mid-value); fold into one Engine test |
| TestResolveHonorsLoadExternalToggle | ruby-api | Global LoadExternal toggle affecting Resolve dies; CLI --no-external-facts is the surviving contract (needs-new item 7) |
| TestSearch_registersCustomFactDirectory | engine | Custom dir registration is reflected in lookup (SearchPath getter dies; registration->resolution survives as Engine option) |
| TestClear_resetsRegisteredSearchPaths | ruby-api | Clear() resets search paths; lifecycle dies with global state |

## Benchmarks (not counted above)

| Benchmark | Disposition |
|---|---|
| BenchmarkExpandCommand | ruby-api; move with internal executor if ExpandCommand survives internally |
| BenchmarkValueProgrammaticCustomFact | engine; re-home on new lookup API |
| BenchmarkToHash | engine; re-home on Snapshot collection |
| BenchmarkWarnNoHandler | ruby-api; drop (no-handler fast path dies with OnMessage) |

## TestMain

`TestMain` stubs `defaultExternalFactDirs` to return nil so the host's real facts.d cannot leak into tests. The Engine test suite must keep an equivalent seam (internal/app/app_test.go has its own TestMain doing the same via `defaultExternalFactDirs`).

## Engine-row coverage audit (task 5.3)

Every engine-classified row verified after facter_test.go retirement. 76 rows marked engine in the inventory (the two zero-weight WithFact rows were re-classified ruby-api after implementation, per their DIVERGENCE annotations above).

- 24 rows covered by internal/facter tests (custom_test.go for DSL string names/brace+execution setcode/kernel confines/hash-int-bool values, external_test.go for name normalization, FACTERname env parsing, stderr warnings, recursion skip, and null-byte rejection, query_test.go for default-flat vs force-dot dotted-fact selection, core_test.go for ipaddress6/networking aliases, networking.interfaces, networking.primary consistency, load_averages, darwin dmi.product.name, processors-extensions parsing)
- 28 rows covered by engine_test.go (snapshot tree/value digging, nearest-fact rejection, resolved-nil vs missing, WithFact normalization and null-byte handling, scoped custom-file loading, once-only "No facts loaded" diagnostics, external>custom>core precedence, freshness-by-rediscovery, option-registration-to-resolution contracts)
- 5 rows covered by internal/app CLI-contract tests: invalid array-index queries (TestRun_invalidCustomArrayIndexQueriesPrintNothing), default external fact directory (TestRun_queryDefaultExternalFactDirectory), registered dirs replacing defaults (TestRun_externalDirOverridesDefaultExternalFactDirectory), env-external overriding core in full output (TestRun_externalFactOverridesCoreFactInFullJSON), external YAML array digging (TestRun_queryExternalYAMLArrayIndex)
- 17 rows newly covered by engine_contract_test.go:
  - TestSnapshotValue_queriesAreCaseInsensitive — TestValueAndFactDowncaseUserQueryLikeRubyAPI, TestFact_canonicalizesMixedCaseQueries
  - TestSnapshotValue_nullByteQueryIsNotFound — TestValue_rejectsQueryWithNullByte (new API returns ErrFactNotFound instead of panicking)
  - TestSnapshotTree_includesStandardCoreRootFactsAndNetworkAliases — TestToHash_includesStandardCoreRootFacts, TestToHash_includesLegacyNetworkAliases
  - TestWithFact_resolverRunsExactlyOncePerDiscover — TestAdd_resolvesProgrammaticCustomFactLazily, TestValue_missingFactDoesNotResolveUnrelatedProgrammaticCustomFacts, TestValue_reusesResolvedProgrammaticCustomFact (once-per-run half). DIVERGENCE: WithFact resolution is eager — every registered resolver runs once per Discover even when no query names it; the old lazy/on-demand behavior died with the global API
  - TestWithFact_falseValuesAreFoundAndNonStringMapKeysDig — TestValue_reusesResolvedProgrammaticCustomFact (falsey half), TestValue_digsProgrammaticCustomMapWithStringifiedKeys
  - TestWithCustomDirs_dslFactInputContracts — TestLoadCustomFacts_positiveWeightCustomFactOverridesCoreFactByName, TestDefineFact_legacyPatternNameResolvesCustomFact, TestResolve_standaloneFacterValueDoesNotOverridePreviousCustomFact
  - TestWithSystemDefaults_loadsFacterlibCustomFacts — TestValue_loadsCustomFactsFromFacterlib
  - TestWithSystemDefaults_environmentFactsOverrideExternalFactFiles — TestResolve_environmentExternalFactsOverrideExternalFactFiles
  - TestSnapshotTree_includesCustomAndExternalFactsAndOmitsNilFacts — TestToHash_omitsProgrammaticCustomNilFacts, TestToHash_omitsCustomNilFacts, TestToHash_includesRegisteredCustomAndExternalFacts
- 2 rows obsolete with the global API:
  - TestEach_yieldsFlattenedResolvedFactNames — flattened Each() iteration was global-API surface; Snapshot.All() iterates top-level canonical entries by design (pinned by TestSnapshotAll_iteratesSortedTopLevelEntries)
  - TestAdd_allowsNestedValueCallsFromProgrammaticCustomFact — WithFact resolvers receive only a context; the new API has no nested fact-lookup handle, so the no-deadlock contract has no surviving surface

Coverage note: external null-byte values now surface as ErrNullByte discovery failures (pinned by internal TestLoadExternalFacts_rejectsNullBytes) rather than the old warn-and-drop with a "contains a null byte reference" message.
