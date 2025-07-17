# Project Context for Gemini CLI

This document provides essential context for the Gemini CLI agent operating within the `feed-forge` project.

## Current Environment
- **Operating System:** darwin
- **Current Working Directory:** /Users/shrike/projects/feed-forge
- **Date:** Wednesday, July 16, 2025

## Project Structure
```
/Users/shrike/projects/feed-forge/
├───.gitignore
├───.gitmodules
├───CLAUDE.md
├───go.mod
├───go.sum
├───LICENSE
├───README.md
├───refactoring_plan.md
├───Taskfile.yml
├───.claude/
│   └───settings.local.json
├───.git/...
├───.github/
│   └───workflows/
│       └───ci.yml
├───.kiro/
│   ├───hooks/
│   ├───settings/
│   ├───specs/
│   │   └───project-documentation/
│   └───steering/
├───build/
├───cmd/
│   └───feed-forge/
│       └───main.go
├───internal/
│   ├───config/
│   │   └───config.go
│   ├───hackernews/
│   │   ├───api.go
│   │   ├───categorization_test.go
│   │   ├───categorization.go
│   │   ├───config.go
│   │   ├───database.go
│   │   ├───feed.go
│   │   ├───provider.go
│   │   ├───types.go
│   │   ├───configs/
│   │   │   └───domains.json
│   │   └───testdata/
│   │       └───categorization/
│   │           ├───ask_hn.json
│   │           ├───book_mention.json
│   │           ├───case_insensitive_ask_hn.json
│   │           ├───case_insensitive_show_hn.json
│   │           ├───ebook_mention.json
│   │           ├───empty_domain.json
│   │           ├───github_project.json
│   │           ├───nil_mapper.json
│   │           ├───no_special_categorization.json
│   │           ├───pdf_document.json
│   │           ├───show_hn.json
│   │           └───video_content.json
│   ├───pkg/
│   └───reddit/
│       ├───api.go
│       ├───auth.go
│       ├───config.go
│       ├───feed.go
│       ├───provider.go
│       └───types.go
├───llm-shared/
│   ├───LICENSE
│   ├───project_tech_stack.md
│   ├───README.md
│   ├───USAGE.md
│   ├───examples/
│   │   ├───go-project.doc-validator.yml
│   │   ├───node-project.doc-validator.yml
│   │   ├───python-project.doc-validator.yml
│   │   └───rust-project.doc-validator.yml
│   └───utils/
│       ├───jsfuncs.js
│       ├───pyfuncs.py
│       ├───README.md
│       ├───gofuncs/
│       │   └───gofuncs.go
│       └───validate-docs/
│           └───validate-docs.go
├───pkg/
│   ├───api/
│   │   └───client.go
│   ├───config/
│   │   ├───loader.go
│   │   └───README.md
│   ├───database/
│   │   ├───cache.go
│   │   ├───provider_utils.go
│   │   ├───types.go
│   │   ├───utils_test.go
│   │   └───utils.go
│   ├───feed/
│   │   ├───custom_atom.go
│   │   ├───generator.go
│   │   ├───provider_helpers.go
│   │   ├───types_test.go
│   │   ├───types.go
│   │   └───testdata/
│   │       └───escape_xml/
│   │           ├───all_special_chars.xml
│   │           ├───ampersand.xml
│   │           ├───double_quotes.xml
│   │           ├───empty_string.xml
│   │           ├───greater_than.xml
│   │           ├───less_than.xml
│   │           ├───multiple_ampersands.xml
│   │           ├───no_special_chars.xml
│   │           └───single_quotes.xml
│   ├───filesystem/
│   │   ├───utils_test.go
│   │   ├───utils.go
│   │   └───very/
│   │       └───long/
│   │           └───path/
│   │               └───with/
│   │                   └───many/
│   │                       └───segments/
│   │                           └───that/
│   │                               └───goes/
│   │                                   └───quite/
│   │                                       └───deep/
│   │                                           └───into/
│   │                                               └───the/
│   │                                                   └───filesystem/
│   ├───http/
│   │   ├───client_test.go
│   │   ├───client.go
│   │   ├───response_test.go
│   │   ├───response.go
│   │   └───testdata/
│   │       └───json_responses/
│   │           ├───empty_object.json
│   │           ├───error_response.json
│   │           ├───invalid_json.json
│   │           ├───non_json_content.txt
│   │           └───valid_response.json
│   ├───interfaces/
│   │   └───database.go
│   ├───opengraph/
│   │   ├───database.go
│   │   ├───fetcher.go
│   │   └───types.go
│   ├───providers/
│   │   ├───base.go
│   │   ├───provider.go
│   │   └───registry.go
│   ├───testutil/
│   │   └───golden.go
│   └───utils/
│       ├───url_test.go
│       └───url.go
├───plan/
├───scripts/
├───templates/
└───testdata/
    ├───README.md
    ├───fixtures/
    └───golden/
```

## Gemini Added Memories
- Always use "task build" to test and lint the project instead of running "go test" or "go build"
