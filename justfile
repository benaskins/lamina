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
