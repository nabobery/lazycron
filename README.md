# LazyCron

A terminal user interface for managing cron jobs, built with Go and Bubble Tea.

## Features

- **Unified Job View**: See all your cron jobs from multiple sources in one place
- **Schedule Understanding**: Human-readable schedule descriptions and next run times
- **Safe Editing**: Toggle, delete, create, and edit jobs with validation
- **Manual Execution**: Run jobs on demand and see output in real-time
- **System Cron Discovery**: Read-only visibility of system cron sources
- **Document Preservation**: Maintains original formatting, comments, and structure

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/nabobery/lazycron.git
cd lazycron

# Build the binary
go build -o lazycron ./cmd/lazycron

# Install (optional)
sudo mv lazycron /usr/local/bin/
```

### Requirements

- Go 1.25.0 or later
- Access to system `crontab` command
- Terminal that supports TUI applications

## Usage

### Launch Interactive TUI

```bash
lazycron
```

### CLI Commands

```bash
# List all cron jobs
lazycron list

# Validate cron expressions
lazycron validate

# Run a specific job manually
lazycron run <job-id>

# Check system configuration
lazycron doctor
```

### Keybindings

| Key | Action |
|-----|--------|
| `j` / `Down` | Move selection down |
| `k` / `Up` | Move selection up |
| `Enter` | Focus or expand pane |
| `Space` | Toggle enable/disable |
| `e` | Edit selected job |
| `n` | Create new job |
| `d` | Delete selected job |
| `x` | Run selected job now |
| `/` | Open search |
| `r` | Reload from disk |
| `q` / `Esc` | Back or quit |

## Architecture

LazyCron follows a clean architecture pattern with separation of concerns:

```
cmd/lazycron/       # Entry point
internal/
├── app/            # Application services
├── cli/            # Command-line interface
├── cronparse/      # Document-preserving parser
├── domain/         # Core types and validation
├── platform/       # System adapters
│   ├── crontab/    # User crontab operations
│   ├── cronlogs/   # Log providers
│   └── systemcron/ # System cron discovery
├── runner/         # Job execution
├── schedule/       # Schedule calculations
├── testutil/       # Test helpers
└── tui/            # Bubble Tea interface
```

### Key Design Decisions

1. **Document Preservation**: The parser maintains full source fidelity, including comments, blank lines, and formatting
2. **Safe Apply**: All writes go through validation with drift detection
3. **Read-Only Awareness**: System jobs are marked as read-only and cannot be modified
4. **Two-Layer Parser**: Raw document classification → normalized job projection

## Development

### Prerequisites

- Go 1.25.0+
- just (task runner)

### Commands

```bash
# Format code
just fmt

# Run linter
just lint

# Run tests
just test

# Run tests with race detector
just test-race

# Build binary
just build

# Run locally
just run

# Run full check (format, lint, test)
just check

# CI pipeline (check + build)
just ci
```

### Testing

The project includes comprehensive tests for:

- Parser fidelity and line preservation
- Schedule calculations
- Safe apply pipeline
- Job execution
- TUI state transitions

## Supported Platforms

- macOS (latest)
- Ubuntu 24.04 LTS

## Security Considerations

- Jobs are never executed automatically without explicit user action
- All writes validate before applying
- Original crontab is preserved if apply fails
- System jobs are read-only unless explicitly allowed
- Terminal state is restored even on crashes

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow existing code patterns and conventions
- Add tests for new functionality
- Ensure `just check` passes before submitting
- Document any new public APIs

## License

MIT License - see LICENSE file for details

## Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI framework
- Uses [robfig/cron](https://github.com/robfig/cron) for cron parsing
- Inspired by tools like lazygit and lazydocker