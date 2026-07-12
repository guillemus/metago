# JSON reliability test plan

This is the acceptance plan for `std.serde.json`. A checked item must have an executable test and
pass without relying on an accidental `encoding/json` fallback where generated handling is required.
The suite adapts subjects and edge cases rather than copying upstream test implementations.

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

## Go values and containers

- [x] Match missing and `null` behavior for scalars, pointers, slices, arrays, maps, and interfaces.
- [x] Allocate pointers for present non-null values, including nested pointers.
- [x] Distinguish nil from empty slices and maps during encoding.
- [x] Replace/reset slices and merge maps with the correct decode semantics.
- [x] Support fixed arrays, including short and long JSON arrays.
- [x] Encode/decode `[]byte` as base64 and preserve nil/empty distinctions.
- [x] Preserve `json.RawMessage` bytes and copy them when required.
- [x] Support `map[string]T` and named string-key types; generated string-key maps sort encoded keys.
- [ ] Support nested generated structs, their pointers, slices, arrays, and maps.
- [ ] Support named scalar types without falling back to reflection.
- [x] Handle interface fields according to their concrete values and decode policy.
- [x] Detect cycles through pointers to generated types rather than overflowing the stack.

## Custom behavior and failures

- [x] Honor `json.Marshaler` and `json.Unmarshaler` fallback fields.
- [x] Honor `encoding.TextMarshaler` and `encoding.TextUnmarshaler` fallback fields.
- [x] Prefer JSON interfaces over text interfaces when both are implemented.
- [x] Handle nil pointers implementing custom interfaces consistently.
- [x] Propagate custom-interface errors without corrupting output or receiver state.
- [x] Do not partially modify the receiver after any decode failure in generated scalar, pointer, slice,
  and supported-map paths.
- [ ] Return useful field paths and byte offsets without requiring exact `encoding/json` wording.
- [ ] Never silently accept a value with the wrong JSON kind.

## Generated-code guarantees

- [x] Nested annotated fields call generated codecs.
- [x] Configured codecs import and use the shared generated runtime.
- [x] Empty runtime configuration uses package-local runtime symbols.
- [x] Unsupported fields deliberately use `encoding/json` fallback.
- [ ] Named scalars, pointers, arrays, bytes, raw messages, and supported maps use generated paths.
- [ ] Generated imports are minimal, stable, and compile for every supported field combination.
- [ ] Output is deterministic across metago runs.

## Corpus, regression, and fuzzing

- [ ] Adapt all applicable JSONTestSuite `y_` and `n_` subjects into table-driven tests.
- [ ] Review every JSONTestSuite `i_` subject and record an explicit accept/reject policy.
- [ ] Add focused regression subjects discovered in each referenced implementation.
- [ ] Seed decoder fuzzing from the syntax, number, Unicode, and regression tables.
- [ ] Add encoder property tests: output is valid JSON and decoding it preserves the value.
- [ ] Add decoder invariants: no panic/hang, bounded allocation policy, and stable failure state.
- [ ] Run fuzzers under race-enabled and architecture-diverse CI where practical.
