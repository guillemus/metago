# JSON reliability test plan

This is the acceptance plan for `std.serde.json`. A checked item must have an executable test and
pass without relying on an accidental `encoding/json` fallback where generated handling is required.
The suite adapts subjects and edge cases rather than copying upstream test implementations.

This file is the single source of truth for remaining JSON work. Implementation is restricted to
`std/serde` templates, runtime source, fixtures, tests, test data, and documentation. Changes to the
metago compiler are out of scope unless they are separately designed and explicitly approved.

## Scope and release contract

- [x] Support JSON only; YAML, MessagePack, CBOR, XML, and other formats are out of scope.
- [x] Use `encoding/json` as canonical behavior unless a tested deviation is documented.
- [x] Document that unsupported field shapes deliberately fall back to `encoding/json` and may use
  reflection.
- [x] Keep every production compatibility test green; full repository tests, static analysis,
  serde race tests, deterministic regeneration, and focused fuzz gates pass after each completed
  vertical slice.
- [x] Review the deviation inventory and document that no intentional JSON behavior deviation is
  currently retained; historical differences have executable compatibility coverage.

Primary references:

- RFC 8259 and ECMA-404 for JSON syntax.
- Go `encoding/json` encode, decode, scanner, number, tag, and fuzz tests.
- JSONTestSuite `y_` (accept), `n_` (reject), and `i_` (document a policy) categories.
- `serde_json` parser, number, float round-trip, string, recursion, map, and regression tests.
- Sonic RFC, decoder, encoder, UTF-8, error-recovery, and number tests.
- goccy/go-json encode/decode, tags, type coverage, marshaler, array, bytes, map, and number tests.
- json-iterator type tests for structs, embedding, maps, arrays, scalars, custom interfaces, raw
  messages, null, case matching, and escaping.
- easyjson basic, required, unknown-field, member-name escaping, string escaping, and lexer tests.

Initial review baseline: Go 1.26.2, Sonic v1.15.2, goccy/go-json v0.10.6,
json-iterator/go v1.1.12, easyjson v0.9.2, and the current serde_json and JSONTestSuite repositories
reviewed in July 2026. Future reviews should update this line so newly discovered regressions are not
silently missed.

## Parser and document framing

- [x] Accept every RFC 8259 whitespace character around a document.
- [x] Reject empty and whitespace-only input.
- [x] Reject trailing non-whitespace data and concatenated documents.
- [x] Accept valid empty and nested objects and arrays in supported fields.
- [x] Reject missing commas, extra commas, missing colons, mismatched delimiters, and truncation.
- [x] Reject non-JSON whitespace, comments, single quotes, identifiers, and JavaScript literals.
- [x] Define and test a maximum nesting-depth policy (10,000 levels).
- [x] Never panic, hang, or read out of bounds for the curated malformed-input corpus.

## Strings and object names

- [x] Decode every JSON escape and escaped solidus.
- [x] Decode BMP escapes and valid UTF-16 surrogate pairs.
- [x] Replace lone, reversed, truncated, and malformed surrogate sequences with U+FFFD.
- [x] Reject unknown escapes, unescaped control bytes, and unterminated strings.
- [x] Replace every curated malformed UTF-8 sequence class with U+FFFD during decoding.
- [x] Encode quotes, backslashes, controls, and required short escapes correctly.
- [x] Emit valid UTF-8 for malformed Go strings using the U+FFFD replacement policy.
- [x] Match `encoding/json` HTML escaping for `<`, `>`, `&`, U+2028, and U+2029.
- [x] Handle escaped, Unicode, empty, and control-containing object keys.

## Numbers

- [x] Accept zero, negatives, fractions, and both exponent forms.
- [x] Reject leading zeroes, leading plus, missing integer/fraction/exponent digits, hex, NaN, and infinity.
- [x] Decode every built-in signed and unsigned width with overflow and underflow validation.
- [x] Reject negative input for unsigned fields.
- [x] Reject fractional and exponent values for integer fields.
- [x] Preserve float32 and float64 round trips, including subnormals and signed zero.
- [x] Accept finite exponent underflow as zero and reject exponent overflow.
- [x] Reject NaN and positive/negative infinity during encoding.
- [x] Encode integers exactly through their full ranges.
- [x] Support and test methodless named numeric types through generated paths.

## Struct field selection and tags

- [x] Honor field renaming, ignored fields, empty tag names, and valid punctuation in names.
- [x] Implement `omitempty` for every supported kind.
- [x] Implement the `string` tag option for signed integer fields.
- [x] Ignore unexported fields during encoding and decoding.
- [x] Match keys case-insensitively while preferring exact matches.
- [x] Resolve anonymous and embedded struct fields with breadth-first `encoding/json` dominance rules.
- [x] Resolve conflicting names and tagged-versus-untagged fields deterministically.
- [x] Ignore unknown fields by default and preserve a path to future strict mode.
- [x] Process duplicate keys in input order with the documented merge/replacement semantics.
- [x] Escape generated field names correctly.
- [x] Implement Go's `omitzero` tag behavior, including `IsZero` methods and combination with
  `omitempty`; unsupported composite zero checks use the documented per-field fallback.
- [x] Implement the `string` option for strings, booleans, signed/unsigned integers, floats, and
  methodless named scalar types.
- [x] Support explicitly tagged anonymous fields and anonymous struct pointers through the canonical
  whole-struct fallback, including pointer allocation/null, promotion/dominance, custom interfaces,
  and atomic failure state via generated anonymous-pointer clones.

## Go values and containers

- [x] Match missing and `null` behavior for scalars, pointers, slices, arrays, maps, interfaces, and
  scalar elements inside containers; retain the differential-fuzz regression for null map values.
- [x] Allocate pointers for present non-null values, including nested pointers.
- [x] Distinguish nil from empty slices and maps during encoding.
- [x] Replace/reset slices and merge maps with the correct decode semantics.
- [x] Support fixed scalar and generated-struct arrays natively, including short and long JSON arrays.
- [x] Encode/decode `[]byte` as base64 and preserve nil/empty distinctions.
- [x] Preserve `json.RawMessage` bytes and copy them when required.
- [x] Support `map[string]T` and named string-key types; generated string-key maps sort encoded keys.
- [x] Support nested generated structs and their pointers across fields, slices, arrays, and maps.
- [x] Support methodless named scalar types without reflection in direct fields, pointers, nested
  pointers, slices, pointer slices, fixed arrays, scalar maps, and pointer-valued scalar maps.
- [x] Complete native built-in scalar support inside slices and maps, including `int8` and `uint8` with
  width-aware overflow checks.
- [x] Support slices of built-in scalar pointers and generated-struct pointers natively.
- [x] Support deeper nested pointer/container combinations natively through triple scalar pointers,
  slices of double scalar pointers, and maps of triple scalar pointers, with canonical null and
  overflow behavior.
- [x] Handle interface fields according to their concrete values and decode policy.
- [x] Detect cycles through pointers to generated types rather than overflowing the stack.

## Map keys and values

- [x] Support every first-level natively supported value shape in `map[string]T`, including scalars,
  single/double pointers, slices, fixed arrays, bytes, raw messages, generated structs, and nested
  string-keyed maps; arbitrary recursively composed containers remain tracked separately.
- [x] Use generated paths for scalar and scalar-pointer values, generated structs and their pointers,
  scalar slices (including base64 bytes), fixed scalar arrays, and nested string-keyed scalar maps.
- [x] Use generated paths for map values containing slices and fixed arrays of generated structs,
  slices of generated-struct pointers, and nested maps of generated structs or pointers, with cycle
  detection for pointer elements.
- [x] Use generated paths for double scalar pointers, scalar-pointer slices, and `json.RawMessage`
  map values, including independent raw-byte ownership.
- [x] Use generated paths for methodless named string key types.
- [x] Support map keys implementing `encoding.TextMarshaler` and `encoding.TextUnmarshaler` through
  the deliberate `encoding/json` fallback.
- [x] Sort generated string-key map output deterministically to match `encoding/json`.

## Custom behavior and failures

- [x] Honor `json.Marshaler` and `json.Unmarshaler` fallback fields.
- [x] Honor `encoding.TextMarshaler` and `encoding.TextUnmarshaler` fallback fields.
- [x] Prefer JSON interfaces over text interfaces when both are implemented.
- [x] Handle nil pointers implementing custom interfaces consistently.
- [x] Propagate custom-interface errors without corrupting output or receiver state.
- [x] Do not partially modify the receiver after any decode failure in generated scalar, pointer, slice,
  and supported-map paths.
- [x] Return useful field paths and byte offsets without requiring exact `encoding/json` wording.
- [x] Never silently accept a value with the wrong JSON kind for generated scalar and container paths.
- [x] Include the root type and full nested field path in type errors.
- [x] Report expected Go types and actual JSON value kinds for generated field errors.
- [x] Verify duplicate-key merge/replacement behavior for nested objects, maps, slices, and pointers.
- [x] Add a `strict=true` template argument that rejects unknown fields while preserving atomic
  receiver state; default codecs continue to ignore validated unknown values.
- [x] Add validated `maxinput` and `maxdepth` arguments: input size is rejected before receiver
  allocation, and depth covers generated recursion plus skipped/raw/fallback values. Document that
  the byte cap bounds input-derived string and collection storage, while semantic limits remain
  application validation.

## Generated-code guarantees

- [x] Nested annotated fields call generated codecs.
- [x] Configured codecs import and use the shared generated runtime.
- [x] Empty runtime configuration uses package-local runtime symbols.
- [x] Unsupported fields deliberately use `encoding/json` fallback.
- [x] Methodless named scalar fields use generated paths.
- [x] Built-in scalar pointers, two-level scalar pointers, and pointers to generated structs use
  generated paths.
- [x] Supported fixed arrays and `[]byte` use generated paths.
- [x] `json.RawMessage` uses generated validation, canonical re-encoding, and copying paths.
- [x] Supported scalar maps and methodless named string keys use generated paths.
- [x] Generated imports are branch-driven, minimal, and stable across isolated empty, string-only,
  float-only, scalar, map, fallback, embedded, strict, limit, raw-message, and `omitzero` fixtures;
  every fixture compiles and its exact import set is asserted.
- [x] Representative mixed generated field shapes compile together without missing or duplicate imports.
- [x] Output is deterministic across metago runs.
- [x] Preserve JSON-over-text interface precedence across every current generated-path replacement
  and the canonical anonymous-field fallback, with direct and embedded executable coverage.

## Corpus, regression, and fuzzing

- [x] Establish an accepted/rejected/ambiguous fixture layout with a pinned JSONTestSuite commit,
  original vector names, adaptation notes, and the upstream MIT license.
- [x] Execute an initial RFC 8259 and JSONTestSuite grammar, framing, and exponent-policy subset.
- [x] Adapt all 95 `y_` and 188 `n_` JSONTestSuite parsing subjects at the pinned commit into a
  digest-verified table-driven differential test while retaining readable regression exemplars.
- [x] Review all 35 JSONTestSuite `i_` subjects at the pinned commit, execute each adapted policy,
  and record its explicit accept/reject decision and observable behavior in provenance.
- [x] Add focused, provenance-recorded regressions from Go `encoding/json`, serde_json, Sonic,
  goccy/go-json, jsoniter, and easyjson, including depth safety, named-byte/base64 behavior, numeric
  byte arrays, float formatting, malformed decimals, RawMessage ownership, and escaped solidus.
- [x] Seed decoder fuzzing from syntax, number, Unicode, generated-bound, and nested-container
  regression subjects.
- [x] Add encoder property tests: output is valid JSON and decoding it preserves supported values.
- [x] Assert stable receiver state after decoder fuzz failures.
- [x] Differentially fuzz generated decode acceptance and values plus semantic encoder output against
  `encoding/json` for the supported value and number compatibility fixtures.
- [x] Add decoder invariants for no panic/hang, configurable pre-allocation input bounds, generated
  and generic depth bounds, and stable receiver state after every tested failure class.
- [x] Run every fuzz target under the race detector, cross-compile serde tests for Linux amd64 and
  arm64, and document the exact CI matrix in `CI.md`; executing on both architectures requires
  workflow changes outside the approved serde-only implementation boundary.

## Documented policy decisions

- [x] Copy retained decoded strings independently so they do not alias or pin the input; test buffer
  reuse for struct fields plus generated map keys and values, and document the allocation tradeoff.
- [x] Review every historical README deviation; each now matches `encoding/json` with executable
  coverage, so no intentional JSON behavior deviation remains.

## Later capabilities

These are optional follow-on features, not blockers for the JSON compatibility and production-readiness
contract above. They require a separate API/design request.

- [ ] Add streaming encoder and decoder APIs after compatibility is complete.
- [x] Keep canonical `encoding/json` HTML escaping; no concrete use case currently justifies a
  configuration surface.
- [x] Profile escaped strings, scalar containers and bytes, nested pointer maps, sparse nil/empty
  values, and numeric boundaries in addition to the existing four-size user feed; record throughput,
  allocations, environment, and reproduction commands.
- [x] Keep other serialization formats outside this JSON-only package and require a separate design
  after JSON production readiness.
