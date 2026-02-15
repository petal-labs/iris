# Contributing to Iris

Thanks for contributing to Iris.

## Prerequisites

- Go 1.24+
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

- Format with `gofmt` (`make fmt`).
- Keep `go vet` clean (`make vet`).
- Prefer small, composable functions over large control blocks.
- Add tests for behavior changes and regressions.

## Pull Request Checklist

- [ ] Scope is focused and described clearly.
- [ ] `make lint`, `make test`, and `make build` pass locally.
- [ ] Integration tests were run when relevant, or explicitly noted as not run.
- [ ] New behavior is covered by tests.
- [ ] User-facing docs are updated when behavior or APIs change.
