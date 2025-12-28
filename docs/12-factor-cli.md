# 12-Factor CLI App Principles

Reference for building well-behaved CLI tools. Based on https://medium.com/@jdxcode/12-factor-cli-apps-dd3c227a0e46

## 1. Great Help is Essential
- `-h` and `--help` must work
- Show examples, not just options
- Group related flags

## 2. Prefer Flags to Args
- Flags are self-documenting: `--output dir` vs positional `dir`
- Use args for the primary noun (file to process)
- Flags for everything else

## 3. What Version Am I On?
- `-v` or `--version` must work
- Include commit hash in builds
- Show dependency versions if relevant

## 4. Mind the Streams
- stdout for output (data, results)
- stderr for logs, progress, errors
- This enables piping: `go-split analyze file.go | jq`

## 5. Handle Signals
- Ctrl+C (SIGINT) should clean up gracefully
- Write partial results if possible
- Don't leave temp files behind

## 6. Use Exit Codes Properly
- 0 = success
- 1 = general error
- 2 = misuse (bad args)
- Use distinct codes for distinct failures

## 7. Be Fancy (When Appropriate)
- Colors, spinners, progress bars for interactive use
- Detect TTY: disable fancy output for pipes
- `NO_COLOR` env var support

## 8. Prompt Carefully
- Avoid interactive prompts in automation
- `--yes` or `--force` to skip confirmation
- Read from stdin when piped

## 9. Remember State Thoughtfully
- Config in `~/.config/appname/` (XDG_CONFIG_HOME)
- Cache in `~/.cache/appname/` (XDG_CACHE_HOME)
- Environment variables for temporary overrides

## 10. Be Chatty (But Not Too Much)
- Explain what's happening
- `-q/--quiet` for silent mode
- `-v/--verbose` for debug info
- Default: show key steps

## 11. Speed is a Feature
- Sub-100ms startup for simple commands
- Show progress for long operations
- Fail fast on bad input

## 12. Scripting-Friendly
- JSON output option (`--json`)
- Parseable output for automation
- Consistent formatting

---

## Quick Checklist

```
[ ] -h/--help shows useful help with examples
[ ] -v/--version shows version
[ ] Uses stdout for data, stderr for messages
[ ] Exit codes are meaningful
[ ] Handles Ctrl+C gracefully
[ ] Works in pipes (no fancy output when not TTY)
[ ] Config via env vars and/or config file
[ ] Has --quiet and --verbose options
[ ] Fast startup
[ ] --json for machine-readable output
```
