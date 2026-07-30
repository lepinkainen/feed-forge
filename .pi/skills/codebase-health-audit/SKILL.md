---
name: codebase-health-audit
description: Audits any codebase for health issues (large modules, duplication, dead/unused code, coverage gaps, and doc/config drift) and reports findings with file paths, line numbers, evidence, and impact.
---

# Codebase Health Audit

Use this skill when asked to assess repository health, including large files/modules, duplicated logic, dead/legacy code, unused params/flags, test coverage gaps, and documentation/config drift.

## Workflow

1. **Capture working tree state**
   - `git status --short` to include uncommitted changes.

2. **Identify large files/modules**
   - For Go/TS/JS/Python/Java/etc., run size scans in relevant roots:
     - Example: `find cmd internal pkg docs -type f -name '*.go' -o -name '*.md' | xargs wc -l | sort -nr | head -n 20`
   - Flag files >= 400 LOC and call out potential refactors.

3. **Scan for duplication and dead/legacy code**
   - Use `rg` to find repeated blocks, duplicated constants, or similar functions.
   - Check for `TODO`/`deprecated`/`removed` comments.
   - Search for unreferenced functions/flags/config keys with `rg` and `go list`/`go test` awareness.

4. **Map tests to modules**
   - `find . -type f -name '*_test.*'` to list tests.
   - Compare module locations to tests; highlight missing coverage for critical paths (CLI flags, error handling, integrations).

5. **Check documentation/config drift**
   - Compare README/config templates with actual code behavior and CLI flags.
   - Verify any config examples match real config structures and defaults.

6. **Collect evidence and line numbers**
   - Use `read` (not `cat`) to inspect files.
   - Use `rg -n` or `nl -ba` for line numbers.

## Output Format

Provide results ordered by severity:

- **Severity** (High/Medium/Low)
- **File path + line number**
- **Brief evidence**
- **Impact**

Then list:

- **Coverage gaps** (tests vs modules)
- **Doc/config mismatches**

If none, say so and mention residual risks.

## Notes

- Be concise and include file paths clearly.
- When the repo has a task/issue system, do not create TODOs; follow the project’s issue workflow if asked.
