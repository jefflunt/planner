# Building `planner`

This repository uses a set of Bash scripts to manage building and testing.

## The `script/build` script

The easiest way to build the project is to run:

```bash
./script/build
```

### What it does:

1. Computes an automatic version string based on `git rev-list --count HEAD`, your `GOARCH`, and checks if the working directory is "dirty" (`-dev` suffix).
2. Uses the `-ldflags "-X ..."` flag to inject this version string into `pkg/version.Version`.
3. Compiles `cmd/plan-tui`.
4. Outputs the resulting executable into the `bin/` directory at the root of the project.

## The `script/test` script

To run the full test suite for the orchestrator, run:

```bash
./script/test
```

This runs `go test -v ./...` against all subpackages.
