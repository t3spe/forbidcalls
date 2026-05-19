# forbidcalls

Configurable Go static analyzer that flags references to specific functions, methods, or whole-package wildcards. Useful for enforcing "use this single wrapper, not the raw stdlib call" policies — e.g. banning direct `os.Getenv` everywhere except one designated config reader.

## What it does

- Matches references via the type checker (`pass.TypesInfo`), so renamed imports (`import foo "os"`), dot imports, and value captures (`f := os.Getenv`) are all caught.
- Three pattern forms: exact (`os.Getenv`), package wildcard (`os.*`), method receivers (`(*net/http.Client).Do`).
- File-level exclusion via doublestar globs.
- Line-level escape hatch via `//forbidcalls:ignore` (inline or leading comment).
- Ships as both a standalone CLI and a [golangci-lint module plugin](https://golangci-lint.run/plugins/module-plugins/).

## Install

### As a golangci-lint plugin (recommended)

`.custom-gcl.yml`:

```yaml
version: v2.12.1            # your golangci-lint version
name: custom-gcl
destination: ./bin
plugins:
  - module: github.com/t3spe/forbidcalls
    version: v0.1.1
```

Build a custom golangci-lint binary, then run it:

```sh
golangci-lint custom
./bin/custom-gcl run ./...
```

Configure the linter in `.golangci.yml`:

```yaml
version: "2"
linters:
  enable:
    - forbidcalls
  settings:
    custom:
      forbidcalls:
        type: module
        settings:
          forbid:
            - os.Getenv
            - os.LookupEnv
            - os.Environ
          exclude_files:
            - internal/config/env.go
```

### As a standalone CLI

```sh
go install github.com/t3spe/forbidcalls/cmd/forbidcalls@v0.1.1
```

Or from source:

```sh
git clone git@github.com:t3spe/forbidcalls.git
cd forbidcalls
make build       # produces ./forbidcalls
```

## Example: ban direct environment variable access

Goal: force every env read through a single `internal/config/env.go` so reads are auditable, testable, and easy to swap (mock, TOML, secrets manager) later.

`.forbidcalls.yaml`:

```yaml
forbid:
  - os.Getenv
  - os.LookupEnv
  - os.Environ
exclude_files:
  - internal/config/env.go
```

`internal/config/env.go` — the one allowed reader:

```go
package config

import "os"

func ReadEnv(key string) string {
    return os.Getenv(key) // allowed: this file is excluded
}
```

`cmd/server/main.go` — violator:

```go
package main

import "os"

func main() {
    port := os.Getenv("PORT")
    _ = port
}
```

Run:

```sh
forbidcalls -config=.forbidcalls.yaml ./...
```

Output:

```
cmd/server/main.go:6:13: forbidden reference to os.Getenv (pattern: os.Getenv)
```

The fix is to route through the wrapper:

```go
import "myrepo/internal/config"

port := config.ReadEnv("PORT")
```

For a one-off escape (e.g. bootstrap before config is loaded), use the line directive:

```go
home := os.Getenv("HOME") //forbidcalls:ignore -- bootstrap path
```

## Pattern syntax

| Form                  | Matches                                                |
| --------------------- | ------------------------------------------------------ |
| `pkg.Name`            | Exact reference to identifier `Name` in package `pkg`  |
| `pkg.*`               | Any exported identifier in package `pkg`               |
| `(*pkg.Type).Method`  | Pointer-receiver method `Method` on `pkg.Type`         |
| `(pkg.Type).Method`   | Value-receiver method `Method` on `pkg.Type`           |

Package paths use the full Go import path: `net/http`, not `http`.

Matching is by resolved `types.Object`, so all of these are flagged identically:

```go
import "os"
import foo "os"
import . "os"

_ = os.Getenv("X")
_ = foo.Getenv("X")
_ = Getenv("X")
f := os.Getenv; f("X")
```

## Exclusions

**File-level.** `exclude_files` is a list of doublestar globs, matched against the absolute file path and against any path suffix — so `internal/config/env.go` matches `/abs/repo/internal/config/env.go`.

```yaml
exclude_files:
  - internal/config/env.go
  - "**/*_test.go"
  - vendor/**
```

**Line-level.** `//forbidcalls:ignore` suppresses violations on the line of the associated statement. Works inline or as a leading comment:

```go
//forbidcalls:ignore -- reason here
home := os.Getenv("HOME")

key := os.Getenv("KEY") //forbidcalls:ignore -- another reason
```

The directive attaches to the nearest AST node via `ast.CommentMap`, so unrelated statements on adjacent lines are not affected.

## Building from source

Go is pinned via `mise.toml`. With [mise](https://mise.jdx.dev/) installed:

```sh
make all       # tidy + fmt + lint + test + build
make test
make build
```

Or directly:

```sh
go test ./...
go build -o forbidcalls ./cmd/forbidcalls
```
