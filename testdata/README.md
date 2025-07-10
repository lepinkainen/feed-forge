# Test Data

This directory contains test data and fixtures for the feed-forge project.

## Directory Structure

- `fixtures/` - Test input data (JSON files, HTML samples, etc.)
- `golden/` - Expected test outputs for golden file testing

## Usage

Tests can reference files in this directory using relative paths like:

```go
testdata := filepath.Join("testdata", "fixtures", "sample.json")
```

## Guidelines

- Keep test files small and focused
- Use descriptive filenames
- Add comments in JSON files where appropriate
- Golden files should be verified manually before committing
