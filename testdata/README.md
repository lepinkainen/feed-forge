# Test Data

This directory holds fixtures that more than one package uses.

Golden files are not stored here. Each package keeps its golden files in a `testdata/`
directory beside the code under test, for example:

- `pkg/feed/testdata/`
- `pkg/preview/testdata/`
- `internal/lobsters/testdata/`
- `internal/bulletin/testdata/`

Put a fixture here only when two or more packages read it. Otherwise put it in the
`testdata/` directory of the package that uses it.

## Usage

Reach a fixture with a relative path:

```go
path := filepath.Join("testdata", "sample.json")
```

## Guidelines

- Keep fixtures small. One fixture covers one case.
- Use descriptive file names.
- To refresh golden files, run `task update-golden`.
- Read the golden file diff before you commit it. The diff is the change in expected
  output.
