# stracetest

[![Go Reference](https://pkg.go.dev/badge/github.com/roy2220/stracetest.svg)](https://pkg.go.dev/github.com/roy2220/stracetest)
[![Coverage](./.badges/coverage.svg)](#)

Package `stracetest` runs a Go subtest under [strace](https://strace.io/)
with syscall fault injection enabled. It lets you exercise error paths that
are otherwise hard to trigger deterministically, such as permission errors,
disk-full conditions, and I/O failures.

> [!NOTE]
> Except for the documentation, every line of code is human-crafted. **No AI slop included**.

## Requirements

- Linux with permission to trace processes.
- `strace` in `PATH`. If `strace` is not found, the subtest is skipped.
- Go 1.25 or newer.

## Quick start

```go
package stracetest_test

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/roy2220/stracetest"
)

func TestExample(t *testing.T) {
	stracetest.Run(t, "open error injected",
		// tempDir is the same directory {{TempDir}} expands to below.
		func(t *testing.T, tempDir string) {
			_, err := os.ReadFile(tempDir + "/foo.txt")

			t.Logf("got error: %v", err)

			if !errors.Is(err, syscall.EACCES) {
				t.Fatal("expected EACCES error")
			}
		},
		// Limit injection to this file; otherwise it hits every openat.
		"--trace-path={{TempDir}}/foo.txt",
		"-e", "trace=openat",
		"-e", "inject=openat:error=EACCES",
		"--quiet=all",
	)

	stracetest.Run(t, "write error injected",
		// tempDir is the same directory {{TempDir}} expands to below.
		func(t *testing.T, tempDir string) {
			err := os.WriteFile(tempDir+"/bar.txt", []byte("bar"), 0644)

			t.Logf("got error: %v", err)

			if !errors.Is(err, syscall.ENOSPC) {
				t.Fatal("expected ENOSPC error")
			}
		},
		// Limit injection to this file; otherwise it hits every write.
		"--trace-path={{TempDir}}/bar.txt",
		"-e", "trace=write",
		"-e", "inject=write:error=ENOSPC",
		"--quiet=all",
	)
}
```

### `{{TempDir}}`

Every `straceArgs` entry may contain `{{TempDir}}`, which is replaced with the
subtest's temporary directory (the `tempDir` passed to your function, removed
automatically afterwards). Use it with `--trace-path` to scope fault injection
to your own files, so the Go runtime and test framework are not affected.

## License

MIT
