# Serde validation gates

Run the portable correctness, static-analysis, and race gates from the repository root:

```sh
go test ./...
staticcheck ./...
go test -race ./std/serde
```

Run each fuzz target under the race detector separately so failures retain a focused corpus:

```sh
go test -race ./std/serde -run '^$' -fuzz '^FuzzCompatibilityValuesUnmarshal$' -fuzztime=30s
go test -race ./std/serde -run '^$' -fuzz '^FuzzCompatibilityValuesDifferential$' -fuzztime=30s
go test -race ./std/serde -run '^$' -fuzz '^FuzzCompatibilityValuesMarshalRoundTrip$' -fuzztime=30s
go test -race ./std/serde -run '^$' -fuzz '^FuzzCompatibilityNumbersDifferential$' -fuzztime=30s
go test -race ./std/serde -run '^$' -fuzz '^FuzzCompatibilityAnonymousDifferential$' -fuzztime=30s
go test -race ./std/serde -run '^$' -fuzz '^FuzzCompatibilityAnonymousPromotionDifferential$' -fuzztime=30s
```

The serde package and generated runtime must also compile for both supported 64-bit architectures:

```sh
GOOS=linux GOARCH=amd64 go test -c -o /tmp/metago-serde-linux-amd64.test ./std/serde
GOOS=linux GOARCH=arm64 go test -c -o /tmp/metago-serde-linux-arm64.test ./std/serde
```

Executing those binaries on both architectures belongs in repository CI. Workflow files live
outside the user-approved `std/serde` implementation boundary, so this package records the exact
matrix without modifying `.github`.
