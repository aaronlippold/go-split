# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of go-split CLI
- `analyze` command - Get AI recommendations for splitting Go files
- `generate` command - Automatically generate split files
- `validate` command - Check Go syntax validity
- `check` command - Run quality checks (fmt, vet, lint, security, tests)
- Direct Anthropic API support via `ANTHROPIC_API_KEY`
- Wrapper mode for use with claude-code-openai-wrapper
- API request/response capture mode for debugging
- Skip flags for individual quality checks
- JSON output mode for scripting
- Test file detection and awareness

## [0.1.0] - 2025-12-28

### Added
- Initial development release
- Core CLI structure following 12-factor CLI principles
- Go AST parsing for file analysis
- Integration with Anthropic Claude API
- Retry with exponential backoff for API calls
- Golden test files for integration testing

[Unreleased]: https://github.com/aaronlippold/go-split/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/aaronlippold/go-split/releases/tag/v0.1.0
