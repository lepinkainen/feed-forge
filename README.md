# Feed Forge

A unified tool to generate RSS feeds from various sources

## Features

- **Hacker News Feed**: Generate RSS feeds from Hacker News top stories
- **Reddit Feed**: Generate RSS feeds from Reddit posts
- **Unified CLI**: Single command-line interface for both providers
- **Configurable**: YAML configuration file support
- **Provider Architecture**: Extensible design for adding new feed sources

## Installation

### From Source

```bash
git clone https://github.com/lepinkainen/feed-forge.git
cd feed-forge
task build
```

### Using Go

```bash
go install github.com/lepinkainen/feed-forge/cmd/feed-forge@latest
```

## Usage

### Generate Reddit Feed

```bash
# Generate Reddit feed with default settings
./build/feed-forge reddit -o reddit.xml

# Generate Reddit feed with custom filters
./build/feed-forge reddit -o reddit.xml --min-score 100 --min-comments 20
```

### Generate Hacker News Feed

```bash
# Generate Hacker News feed with default settings
./build/feed-forge hacker-news -o hackernews.xml

# Generate Hacker News feed with custom filters
./build/feed-forge hacker-news -o hackernews.xml --min-points 100 --limit 20
```

### Configuration

Create a `config.yaml` file to configure the providers:

```yaml
reddit:
  client_id: "your-reddit-client-id"
  client_secret: "" # Leave empty for installed apps
  redirect_uri: "http://localhost:8080/callback"
  feed_type: "atom"
  enhanced_atom: true
  output_path: "reddit.xml"
  score_filter: 50
  comment_filter: 10

hackernews:
  min_points: 50
  limit: 30
```

### Command Line Options

```bash
# Global options
--config string    Configuration file path (default "config.yaml")

# Reddit specific options
--min-score int      Minimum post score (default 50)
--min-comments int   Minimum comment count (default 10)
-o, --outfile string Output file path (default "reddit.xml")

# Hacker News specific options
--min-points int     Minimum points threshold (default 50)
--limit int          Maximum number of items (default 30)
-o, --outfile string Output file path (default "hackernews.xml")
```

## Building

This project uses [Task](https://taskfile.dev/) for build automation:

```bash
# Build the application
task build

# Run tests
task test

# Run linter and formatter
task lint

# Clean build artifacts
task clean

# Build for Linux
task build-linux

# Run Reddit feed generation
task run-reddit

# Run Hacker News feed generation
task run-hackernews
```

## Architecture

- **Provider Interface**: Common interface for all feed providers
- **Unified Configuration**: Single YAML configuration file using Viper
- **Modular Design**: Separate packages for each provider
- **Shared Logic**: Common functionality abstracted to shared packages

### Directory Structure

```
feed-forge/
├── cmd/feed-forge/          # Main application entry point
├── internal/
│   ├── config/              # Configuration management
│   ├── hackernews/          # Hacker News provider
│   ├── reddit/              # Reddit provider
│   └── pkg/providers/       # Provider interface
├── pkg/                     # Shared packages
│   ├── opengraph/          # OpenGraph metadata handling
│   └── feed/               # RSS/Atom feed generation
└── llm-shared/             # Development guidelines submodule
```

## Requirements

- Go 1.24 or later
- Reddit API credentials (for Reddit feeds)
- Internet connection for fetching data

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and linting: `task test lint`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
