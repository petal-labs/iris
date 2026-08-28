# Contributing to Iris

Thanks for contributing to Iris.

By participating, you agree to follow our [Code of Conduct](CODE_OF_CONDUCT.md).

## Prerequisites

- Go 1.25+
- Make (optional, but recommended)
- Git

## Local Setup

```bash
git clone https://github.com/petal-labs/iris.git
cd iris
make install-hooks
```

## Development Workflow

1. Create a branch from `main`.
2. Keep changes focused and small.
3. Run the appropriate test tier locally.
4. Open a pull request with a clear summary and test notes.

## Test Tiers

Use the lightest tier that provides confidence for your change:

### Fast (iteration while coding)

```bash
go test ./core/... ./tools/... ./cli/...
```

### Standard (required before opening PR)

```bash
make lint
make test
make build
```

### Full (networked integration coverage)

```bash
make test-integration
```

Integration tests use real provider APIs and require credentials. See the next section for required environment variables.

## Integration Test Keys

Set provider keys as needed for the tests you run:

```bash
export OPENAI_API_KEY=...
export ANTHROPIC_API_KEY=...   # optional unless running Anthropic tests
export GEMINI_API_KEY=...      # optional unless running Gemini tests
export XAI_API_KEY=...         # optional unless running xAI tests
export ZAI_API_KEY=...         # optional unless running Z.ai tests
export PERPLEXITY_API_KEY=...  # optional unless running Perplexity tests
export VOYAGEAI_API_KEY=...    # optional unless running VoyageAI tests
export HF_TOKEN=...            # optional unless running Hugging Face tests
```

Run a focused OpenAI smoke test:

```bash
go test -tags=integration ./tests/integration/... -run '^TestOpenAI_ChatCompletion$' -count=1 -v
```

In CI, integration tests fail on missing required keys unless `IRIS_SKIP_INTEGRATION=1` is set.

## Code Style and Quality

- Run `make lint` before opening a pull request. It runs the same format, vet,
  and golangci-lint checks enforced by CI.
- Install the pinned golangci-lint `v2.5.0` release with
  `make install-golangci-lint`. `make lint` fails with an installation hint if
  the binary is missing or its version differs from CI.
- To use a binary outside `PATH`, run
  `make lint GOLANGCI_LINT=/path/to/golangci-lint`.
- Format with `gofmt` (`make fmt`) and keep `go vet` clean (`make vet`).
- Prefer small, composable functions over large control blocks.
- Add tests for behavior changes and regressions.

## Change Documentation

Every repository change must create or update one change record under
`docs/changes/`. These files feed the automated documentation pipeline and are
part of the implementation, not an optional follow-up.

Use this filename convention:

```text
docs/changes/YYYY-MM-DD_v{version}_{feature-slug}.md
```

- Use the current UTC date.
- Use the semantic version declared by the nearest module or `VERSION` file;
  use `v0.0.0-dev` when no release version is declared.
- Use a specific kebab-case feature slug.
- Keep related edits in one record; use separate records for unrelated work.
- Do not overwrite another feature's record. If the same feature record already
  exists for the day, add a timestamped revision section.

Every record must include YAML front matter with `date`, `version`, `feature`,
`product`, `change_type`, exhaustive `affected_components`, and `related_frds`.
Use `product: iris`; valid change types are `feature`, `bugfix`, `breaking`,
`deprecation`, `refactor`, `schema`, `api`, `cli`, `docs`, and
`infrastructure`.

Include all of these sections, writing `N/A` with a brief reason when a section
does not apply:

1. Summary
2. Motivation
3. What Changed (New Additions, Modifications, and Removals)
4. Technical Specification
5. Usage Examples
6. Integration Notes
7. Breaking Changes & Migration
8. Deferred / Out of Scope
9. Testing Notes

Be concrete: name changed APIs and files, include exact signatures or schemas
when applicable, explain intent and compatibility, and record the checks that
actually ran. Existing records in `docs/changes/` are useful examples.

## Release Process

Releases are cut from `main` using semantic-version tags. Maintainers should:

1. Open and merge a release-preparation pull request that moves the relevant
   `CHANGELOG.md` entries from `Unreleased` into a `X.Y.Z` section dated in UTC,
   confirms all required `docs/changes/` records are present, and passes CI.
2. Update local `main` and confirm the release commit:

   ```bash
   git checkout main
   git pull --ff-only
   git status --short
   ```

3. Create and push an annotated semantic-version tag. Never move or reuse a
   published tag:

   ```bash
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin vX.Y.Z
   ```

4. The tag triggers `.github/workflows/release.yml`, which builds Linux, macOS,
   and Windows CLI binaries, generates SHA-256 checksums, and creates the GitHub
   Release with generated notes.
5. Verify the workflow succeeded and the release contains all four binaries
   plus `checksums.txt`. If it fails, fix the cause on `main` and cut a new
   patch version; do not replace the published tag.

## Pull Request Checklist

- [ ] Scope is focused and described clearly.
- [ ] `make lint`, `make test`, and `make build` pass locally.
- [ ] Integration tests were run when relevant, or explicitly noted as not run.
- [ ] New behavior is covered by tests.
- [ ] A complete `docs/changes/` record is added or updated.
- [ ] User-facing docs are updated when behavior or APIs change.
