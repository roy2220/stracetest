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
		"--trace=openat",
		"--inject=openat:error=EACCES",
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
		"--trace=write",
		"--inject=write:error=ENOSPC",
		"--quiet=all",
	)
}

func BenchmarkExample(b *testing.B) {
	stracetest.Run(b, "write bypassed",
		// tempDir is the same directory {{TempDir}} expands to below.
		func(b testing.TB, tempDir string) {
			f, err := os.OpenFile(tempDir+"/bar.txt", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { f.Close() })

			data := make([]byte, 1000000)
			for b.(*testing.B).Loop() {
				_, err := f.Write(data)
				if err != nil {
					b.Fatal(err)
				}
			}

			// fi, err := f.Stat()
			// if err != nil {
			//	b.Fatal(err)
			// }
			// if fi.Size() >= 1 {
			//	b.Fatal("expected empty file")
			// }
		},
		// Limit injection to this file; otherwise it hits every write.
		"--trace-path={{TempDir}}/bar.txt",
		"--trace=write",
		"--inject=write:retval=1000000",
		"--quiet=all",
	)
}
