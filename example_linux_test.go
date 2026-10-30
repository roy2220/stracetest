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
		func(t testing.TB, tempDir string) {
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
		func(t testing.TB, tempDir string) {
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
