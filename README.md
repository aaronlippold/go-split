# go-split

[![CI](https://github.com/aaronlippold/go-split/actions/workflows/ci.yml/badge.svg)](https://github.com/aaronlippold/go-split/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/aaronlippold/go-split)](https://github.com/aaronlippold/go-split/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/aaronlippold/go-split)](https://goreportcard.com/report/github.com/aaronlippold/go-split)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Intelligently split large Go files into smaller, focused modules using AI analysis.

## Overview

`go-split` uses Claude AI to analyze Go source files and recommend how to split them into smaller, more maintainable modules. It can:

- **Analyze** files and provide splitting recommendations
- **Generate** split files automatically
- **Validate** Go syntax of generated files
- **Check** code quality (fmt, vet, lint, security, tests)

## Installation

### From Source

```bash
go install github.com/aaronlippold/go-split/cmd/go-split@latest
```

### From Releases

Download the latest binary from [Releases](https://github.com/aaronlippold/go-split/releases).

### Homebrew

```bash
brew tap aaronlippold/tap
brew install --cask go-split
```

## Quick Start

### Option 1: Direct Anthropic API

```bash
# Set your Anthropic API key
export ANTHROPIC_API_KEY=sk-ant-...

# Analyze a file
go-split analyze server.go
```

### Option 2: Claude Code / Wrapper (no API key needed)

If you have [Claude Code](https://claude.ai/code) with the [OpenAI wrapper](https://github.com/RichardAtCT/claude-code-openai-wrapper), you can use go-split without an API key:

```bash
# Start the wrapper (uses your Claude subscription)
claude-code-openai-wrapper

# In another terminal, analyze files
go-split analyze server.go
```

### Basic Workflow

```bash
# Analyze a file
go-split analyze server.go

# Generate split files
go-split generate server.go --output=./split/

# Validate and check quality
go-split validate ./split/
go-split check ./split/
```

## Usage

### API Configuration

go-split can use the Claude API in two ways:

1. **Direct API**: Set `ANTHROPIC_API_KEY` environment variable or use `--api-key`
2. **Wrapper mode** (default): Uses [claude-code-openai-wrapper](https://github.com/RichardAtCT/claude-code-openai-wrapper) at localhost:8000 - works with Claude Code subscription

### Commands

#### Analyze a file

Get AI recommendations for splitting a file:

```bash
go-split analyze server.go
```

#### Generate split files

Automatically generate split files:

```bash
go-split generate server.go --output=./split/
```

Preview without writing:

```bash
go-split --dry-run generate server.go
```

#### Validate generated files

Check that Go files have valid syntax:

```bash
go-split validate ./split/
```

#### Run quality checks

Run fmt, vet, lint, security, and test checks:

```bash
go-split check ./split/
```

Skip specific checks:

```bash
go-split check ./split/ --skip-lint --skip-tests
```

### Flags

| Flag | Description |
|------|-------------|
| `--endpoint URL` | API endpoint (default: http://localhost:8000/v1/messages) |
| `--model NAME` | Model to use (default: claude-sonnet-4-5-20250929) |
| `--api-key KEY` | Anthropic API key (bypasses wrapper) |
| `-V, --verbose` | Verbose output |
| `--dry-run` | Preview without writing files |
| `-o, --output DIR` | Output directory |
| `--capture DIR` | Capture API requests/responses for debugging |
| `--json` | Output in JSON format (for scripting) |
| `--no-color` | Disable colored output |

### Check Flags

| Flag | Description |
|------|-------------|
| `--skip-checks` | Skip all quality checks |
| `--skip-fmt` | Skip gofmt |
| `--skip-vet` | Skip go vet |
| `--skip-lint` | Skip golangci-lint |
| `--skip-sec` | Skip gosec |
| `--skip-build` | Skip go build |
| `--skip-tests` | Skip go test |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `ANTHROPIC_API_KEY` | Direct Anthropic API key (bypasses wrapper) |
| `GO_SPLIT_ENDPOINT` | API endpoint override |
| `GO_SPLIT_MODEL` | Model override |
| `GO_SPLIT_CAPTURE` | Capture directory for debugging |

## Examples

### Using direct API with Haiku (cheaper/faster)

```bash
export ANTHROPIC_API_KEY=sk-ant-...
go-split --model=claude-3-5-haiku-20241022 analyze large_file.go
```

### Capture API calls for debugging

```bash
go-split --capture=./debug/ analyze file.go
ls ./debug/
# 20251228_190000_request.txt
# 20251228_190000_response.txt
```

### Full workflow

```bash
# 1. Analyze and get recommendations
go-split analyze cmd/server/main.go

# 2. Generate split files
go-split generate cmd/server/main.go --output=/tmp/split/

# 3. Validate syntax
go-split validate /tmp/split/

# 4. Run quality checks
go-split check /tmp/split/
```

## Development

### Prerequisites

- Go 1.21+
- golangci-lint (for linting)
- gosec (for security checks)

### Build

```bash
make build
```

### Test

```bash
make test
```

### Lint

```bash
make lint
```

### All checks

```bash
make check
```

## License

Apache-2.0 - See [LICENSE](LICENSE) for details.

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run `make check` to ensure tests pass
5. Submit a pull request

## Related Projects

- [beads](https://github.com/steveyegge/beads) - Git-backed issue tracker for AI agents
- [vc](https://github.com/steveyegge/vc) - AI-powered version control assistant
- [claude-code-openai-wrapper](https://github.com/RichardAtCT/claude-code-openai-wrapper) - OpenAI-compatible wrapper for Claude
