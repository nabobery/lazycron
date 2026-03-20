# CRON PARSER

Document-preserving cron parser with two-layer architecture.

## FILES

| File | Role |
|------|------|
| `parser.go` | Parse → (CronDocument, []CronJob, []ValidationIssue) + Render |
| `parser_test.go` | Fixture-based tests for all line types |

## ARCHITECTURE

1. **Layer 1**: Raw document with classified lines (CronLine)
2. **Layer 2**: Normalized jobs derived from job lines

## KEY FUNCTIONS

```go
Parse(text string, source CronSource) (CronDocument, []CronJob, []ValidationIssue)
Render(doc CronDocument) string  // Round-trip reconstruction
```

## LINE CLASSIFICATION

| Kind | Prefix | Notes |
|------|--------|-------|
| Blank | (empty) | Preserved as-is |
| Comment | `#` | Preserved as-is |
| Env | `KEY=VAL` | Captured in envContext |
| Job | `* * * * *` | Parsed to CronJob |
| Disabled | `# [lazycron-disabled] ` | Restores original on enable |
| Invalid | (malformed) | Captured as ValidationIssue |

## ANTI-PATTERNS

- Never strip comments or blank lines—round-trip fidelity required
- Never normalize whitespace—preserve original formatting
- Never skip invalid lines—surface as warnings
