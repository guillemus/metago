# Repository instructions

## Keep public API documentation synchronized

Whenever changing Metago's directives, target resolution, template data, helpers, configuration, CLI, generated-file behavior, or any other public API:

1. Update `README.md`.
2. Update the relevant pages under `docs/content/`.
3. Update `skills/metago/SKILL.md`.
4. Run `mise run install-skill` after changing the skill so the repository copy is synchronized to the installed agent skill.
5. Add or update tests for the API change, then run `go test ./...` and `staticcheck ./...`.

Do not consider an API change complete while any of these references still describe the old behavior.
