# Curated JSON corpus provenance

The fixtures in `accepted`, `rejected`, and `ambiguous` are adapted from JSONTestSuite rather than
copied wholesale. Review baseline:

- Repository: <https://github.com/nst/JSONTestSuite>
- Commit: `1ef36fa01286573e846ac449e8683f8833c5b26a`
- License: MIT; the upstream license is reproduced in `LICENSE.JSONTestSuite`.
- Reviewed: July 2026.

Generated serde codecs accept objects, so upstream root arrays and scalars are wrapped in a known
`interface` or `float64` fixture field. This preserves the parser behavior under test while making
the vector executable through `CompatibilityValues` or `CompatibilityNumbers`.

`JSONTestSuite-yn.tar.gz.base64` contains the exact 95 `y_` and 188 `n_` parsing subjects from the
pinned commit in a compact, lossless archive. Its decoded SHA-256 is
`165b1ef91bb128d8fec3ffa329f171795e12fc6e3c2609be05257d92ad68b320`. The table-driven test retains
each original filename and wraps every subject as the value of the known `interface` field. All
accepted subjects must decode to the same value as `encoding/json`; all rejected subjects must be
rejected by both decoders. The smaller directories below remain readable regression exemplars.

| Local fixture | Original JSONTestSuite name | Adaptation and policy |
| --- | --- | --- |
| `accepted/y_object_empty.json` | `y_object_empty.json` | Exact JSON subject; trailing newline added. |
| `accepted/y_array_empty.json` | `y_array_empty.json` | Wrapped the root array in `interface`. |
| `accepted/y_structure_lonely_true.json` | `y_structure_lonely_true.json` | Wrapped the root literal in `interface`. |
| `accepted/y_structure_whitespace_array.json` | `y_structure_whitespace_array.json` | Wrapped the root array and retained RFC whitespace around structural tokens. |
| `rejected/n_array_1_true_without_comma.json` | `n_array_1_true_without_comma.json` | Wrapped the malformed array in `interface`. |
| `rejected/n_array_double_comma.json` | `n_array_double_comma.json` | Wrapped the malformed array in `interface`. |
| `rejected/n_object_trailing_comma.json` | `n_object_trailing_comma.json` | Renamed the member to a known fixture field; retained the trailing comma. |
| `rejected/n_object_missing_colon.json` | `n_object_missing_colon.json` | Renamed the member to a known fixture field; retained the missing colon. |
| `rejected/n_structure_double_array.json` | `n_structure_double_array.json` | Wrapped the first array; retained the concatenated trailing document. |
| `rejected/n_incomplete_false.json` | `n_incomplete_false.json` | Wrapped the truncated literal in `interface`. |
| `ambiguous/accept_i_number_real_underflow.json` | `i_number_real_underflow.json` | Moved the number into `float64`; accept as finite underflow to zero, matching `encoding/json`. |
| `ambiguous/reject_i_number_huge_exp.json` | `i_number_huge_exp.json` | Moved into `float64` and shortened the still-overflowing exponent; reject float overflow. |

RFC-derived expectations use RFC 8259 (STD 90, December 2017), especially sections 2–7 for one
complete JSON text, the four allowed whitespace bytes, structural separators, literals, numbers,
and strings. The fixtures paraphrase or adapt behaviors and do not reproduce RFC prose.

## Cross-implementation and security regressions

`regressions_test.go` contains independently written, observable-behavior adaptations rather than
copied upstream test code. The review baseline and exact subjects are:

| Source | Revision and license | Original subject | Local observable behavior |
| --- | --- | --- | --- |
| Go `encoding/json` | Go 1.26.2; BSD-3-Clause | `TestNullString` (issues 2540, 7046, 8587) | JSON `null` leaves non-pointer `,string` scalars unchanged. |
| Go `encoding/json` | Go 1.26.2; BSD-3-Clause | `TestSliceOfCustomByte` (issues 8962, 12921) | Slices whose named element has underlying type `uint8` use byte-slice base64 semantics. |
| Go `encoding/json` | Go 1.26.2; BSD-3-Clause | `TestUnmarshalMaxDepth` | Excessive mixed array/object nesting returns an error without changing the receiver. |
| `serde_json` | commit `827a315bf2198558f0325b07bcc1e2cd973aba2f`; MIT OR Apache-2.0 | `tests/regression/issue953.rs` | A decimal point without a following fraction digit is rejected atomically. |
| `serde_json` | commit `827a315bf2198558f0325b07bcc1e2cd973aba2f`; MIT OR Apache-2.0 | `tests/regression/issue1004.rs` | `float32(5.55)` is formatted at 32-bit precision rather than after widening. |
| Sonic | v1.15.2; Apache-2.0 | `TestSliceOfCustomByte` | Named `uint8` slices round-trip through canonical base64, including map values in the local extension. |
| goccy/go-json | v0.10.6; MIT | `TestIssue360` | Byte slices also accept numeric JSON arrays; `null` elements become zero and overflow is atomic. |
| json-iterator/go | v1.1.12; MIT | `Test_raw_message_memory_not_copied_issue` | Retained `RawMessage` bytes do not alias the decoder input buffer. |
| easyjson | v0.9.2; MIT | `jlexer.TestBytes` | Base64 strings decode after JSON escaped-solidus normalization. |

Repository and license locations reviewed: Go's `LICENSE`, `serde-rs/json`'s `LICENSE-MIT` and
`LICENSE-APACHE`, and each tagged Go module's root `LICENSE`. The local cases retain no upstream
implementation code.

## Complete JSONTestSuite `i_` policy review

`ambiguous_corpus_test.go` executes all 35 subjects present at the pinned commit. Root numbers are
moved into the `float64` fixture field, root arrays are moved into the `interface` field, and invalid
byte sequences are represented losslessly as hexadecimal test bytes. `i_number_huge_exp` uses a
shorter exponent that preserves the original overflow behavior. All other numeric tokens and string
payload bytes are retained. Policies follow `encoding/json` and are checked against it in the test.

| Original subject | Policy | Observable behavior under test |
| --- | --- | --- |
| `i_number_double_huge_neg_exp.json` | Accept | Finite exponent underflow rounds to zero. |
| `i_number_huge_exp.json` | Reject | Positive exponent overflows `float64`. |
| `i_number_neg_int_huge_exp.json` | Reject | Negative value overflows `float64`. |
| `i_number_pos_double_huge_exp.json` | Reject | Positive value overflows `float64`. |
| `i_number_real_neg_overflow.json` | Reject | Negative value overflows `float64`. |
| `i_number_real_pos_overflow.json` | Reject | Positive value overflows `float64`. |
| `i_number_real_underflow.json` | Accept | Finite exponent underflow rounds to zero. |
| `i_number_too_big_neg_int.json` | Accept | Finite integer is represented as a rounded `float64`. |
| `i_number_too_big_pos_int.json` | Accept | Finite integer is represented as a rounded `float64`. |
| `i_number_very_big_negative_int.json` | Accept | Finite integer is represented as a rounded `float64`. |
| `i_object_key_lone_2nd_surrogate.json` | Accept | Lone escaped surrogate is replaced with U+FFFD. |
| `i_string_1st_surrogate_but_2nd_missing.json` | Accept | Incomplete pair is replaced with U+FFFD. |
| `i_string_1st_valid_surrogate_2nd_invalid.json` | Accept | Invalid pair is replaced with U+FFFD. |
| `i_string_UTF-16LE_with_BOM.json` | Reject | JSON input must be UTF-8 and must not start with a BOM. |
| `i_string_UTF-8_invalid_sequence.json` | Accept | Malformed UTF-8 bytes are replaced with U+FFFD. |
| `i_string_UTF8_surrogate_U+D800.json` | Accept | UTF-8 encoding of a surrogate is replaced with U+FFFD. |
| `i_string_incomplete_surrogate_and_escape_valid.json` | Accept | Incomplete surrogate is replaced; following escape remains valid. |
| `i_string_incomplete_surrogate_pair.json` | Accept | Incomplete pair is replaced with U+FFFD. |
| `i_string_incomplete_surrogates_escape_valid.json` | Accept | Both incomplete surrogates are replaced; following escape remains valid. |
| `i_string_invalid_lonely_surrogate.json` | Accept | Lone escaped surrogate is replaced with U+FFFD. |
| `i_string_invalid_surrogate.json` | Accept | Lone escaped surrogate is replaced and following text is retained. |
| `i_string_invalid_utf-8.json` | Accept | Malformed UTF-8 byte is replaced with U+FFFD. |
| `i_string_inverted_surrogates_U+1D11E.json` | Accept | Reversed pair is replaced with two U+FFFD runes. |
| `i_string_iso_latin_1.json` | Accept | Non-UTF-8 byte is replaced with U+FFFD. |
| `i_string_lone_second_surrogate.json` | Accept | Lone low surrogate is replaced with U+FFFD. |
| `i_string_lone_utf8_continuation_byte.json` | Accept | Lone continuation byte is replaced with U+FFFD. |
| `i_string_not_in_unicode_range.json` | Accept | Out-of-range UTF-8 sequence is replaced with U+FFFD. |
| `i_string_overlong_sequence_2_bytes.json` | Accept | Overlong UTF-8 is replaced with U+FFFD. |
| `i_string_overlong_sequence_6_bytes.json` | Accept | Obsolete six-byte sequence is replaced with U+FFFD. |
| `i_string_overlong_sequence_6_bytes_null.json` | Accept | Overlong null sequence is replaced with U+FFFD. |
| `i_string_truncated-utf-8.json` | Accept | Truncated UTF-8 is replaced with U+FFFD. |
| `i_string_utf16BE_no_BOM.json` | Reject | UTF-16BE is not a UTF-8 JSON document. |
| `i_string_utf16LE_no_BOM.json` | Reject | UTF-16LE is not a UTF-8 JSON document. |
| `i_structure_500_nested_arrays.json` | Accept | Depth 500 is below serde's 10,000-level limit. |
| `i_structure_UTF-8_BOM_empty_object.json` | Reject | A leading UTF-8 BOM is not JSON whitespace. |
