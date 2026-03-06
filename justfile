version := `git describe --tags --always --dirty 2>/dev/null || echo dev`

build:
    go build -ldflags "-X main.version={{version}}" -o bin/lamina ./cmd/lamina/

install: build
    cp bin/lamina ~/.local/bin/lamina
    @echo "Installed lamina {{version}}"

test:
    go vet ./cmd/lamina/
    bin/lamina test

# Symlink skills from skills/ into .claude/skills/ for Claude Code discovery
install-skills:
    mkdir -p .claude/skills
    for dir in skills/*/; do \
        name=$(basename "$dir"); \
        ln -sfn "$(pwd)/$dir" ".claude/skills/$name"; \
    done
    @echo "Installed $(ls -1 skills/ | wc -l | tr -d ' ') skill(s) to .claude/skills/"

# Clone, build, and install flux.swift (FLUX.1 image generation on Apple Silicon)
install-flux:
    #!/usr/bin/env bash
    set -euo pipefail
    FLUX_DIR="$HOME/.local/src/flux.swift.cli"
    if [ ! -d "$FLUX_DIR" ]; then
        echo "Cloning flux.swift.cli..."
        mkdir -p "$HOME/.local/src"
        git clone https://github.com/mzbac/flux.swift.cli.git "$FLUX_DIR"
    else
        echo "Updating flux.swift.cli..."
        git -C "$FLUX_DIR" pull --ff-only
    fi
    echo "Building flux.swift.cli (release)..."
    cd "$FLUX_DIR"
    swift build -c release
    cp "$(swift build -c release --show-bin-path)/flux.swift.cli" "$HOME/.local/bin/flux"
    echo "Installed flux to ~/.local/bin/flux"
