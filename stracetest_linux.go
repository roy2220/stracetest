// Package stracetest runs a Go subtest under strace with syscall fault
// injection enabled. It lets you exercise error paths that are otherwise
// hard to trigger deterministically, such as permission errors, disk-full
// conditions, and I/O failures.

package stracetest

import (
	"bufio"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	realTestNameSuffix = "/real"
	fakeTestNameSuffix = "/fake"
)

var (
	testExePath   string
	straceExePath string
	tempDirEnvKey string
)

var initOnce = sync.OnceValue(func() error {
	var err error
	testExePath, err = os.Executable()
	if err != nil {
		return err
	}

	straceExePath, _ = exec.LookPath("strace")

	hash := fnv.New64a()
	hash.Write([]byte(testExePath))
	tempDirEnvKey = "STRACETEST__" + strconv.FormatUint(hash.Sum64(), 36) + "__TEMP_DIR"
	return nil
})

// Run runs a subtest under strace.
func Run(
	tb testing.TB,
	testName string,
	testFunc func(tb testing.TB, tempDir string),
	straceArgs ...string,
) {
	tb.Helper()

	if err := initOnce(); err != nil {
		tb.Fatalf("stracetest: failed to init: %v", err)
	}

	var doRun func(string, func(testing.TB))
	switch v := tb.(type) {
	case *testing.T:
		doRun = func(testName string, testFunc func(testing.TB)) {
			v.Run(testName, func(t *testing.T) {
				testFunc(t)
			})
		}
	case *testing.B:
		doRun = func(testName string, testFunc func(testing.TB)) {
			v.Run(testName, func(b *testing.B) {
				testFunc(b)
			})
		}
	default:
		v.Fatalf("stracetest: %T is not supported", v)
	}

	if straceExePath == "" {
		doRun(testName, func(tb testing.TB) { tb.Skip("stracetest: strace not found in PATH") })
		return
	}

	if tempDir := os.Getenv(tempDirEnvKey); tempDir != "" {
		// is already under strace
		doRun(testName+realTestNameSuffix, func(tb testing.TB) { testFunc(tb, tempDir) })
		return
	}

	doRun(testName+fakeTestNameSuffix, func(tb testing.TB) {
		tb.Helper()

		var straceOutputFile *os.File
		if _, ok := tb.(*testing.T); ok {
			r, w, err := os.Pipe()
			if err != nil {
				tb.Fatalf("stracetest: failed to create pipe: %v", err)
			}
			var wg sync.WaitGroup
			wg.Go(func() {
				scanner := bufio.NewScanner(r)
				for scanner.Scan() {
					tb.Log(scanner.Text())
				}
				_ = scanner.Err()
				io.Copy(io.Discard, r)
			})
			tb.Cleanup(func() {
				w.Close()
				wg.Wait()
				r.Close()
			})
			straceOutputFile = w
		}
		if err := buildTestCmd(tb, straceArgs, straceOutputFile).Run(); err != nil {
			tb.Fatalf("stracetest: failed to run test: %v", err)
		}
	})
}

// DefaultStraceArgs holds strace arguments prepended to those of every [Run]
// call. It should be set in an init function of a *_test.go file, e.g.:
//
//	func init() {
//		stracetest.DefaultStraceArgs = []string{"--quiet=all", "--seccomp-bpf"}
//	}
var DefaultStraceArgs []string

func buildTestCmd(tb testing.TB, straceArgs []string, straceOutputFile *os.File) *exec.Cmd {
	tb.Helper()

	tempDir := tb.TempDir()
	testName := strings.TrimSuffix(tb.Name(), fakeTestNameSuffix) + realTestNameSuffix

	var args []string
	for _, vs := range [...][]string{DefaultStraceArgs, straceArgs} {
		for _, v := range vs {
			args = append(args, strings.ReplaceAll(v, "{{TempDir}}", tempDir))
		}
	}
	args = append(args, "--follow-forks")
	if straceOutputFile == nil {
		args = append(args, "--output=/dev/null")
	} else {
		args = append(args, "--output=/dev/fd/3")
	}
	args = append(args, "--", testExePath)
	args = append(args, os.Args[1:]...)
	// override existing -test.run and -test.count
	args = append(args,
		"-test.run", buildTestRunPattern(testName),
		"-test.count", "1",
	)

	cmd := exec.CommandContext(tb.Context(), straceExePath, args...)
	cmd.Env = append(os.Environ(), tempDirEnvKey+"="+tempDir)
	if straceOutputFile != nil {
		cmd.ExtraFiles = []*os.File{straceOutputFile}
	}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd
}

func buildTestRunPattern(testName string) string {
	var b []byte
	for s := testName; s != ""; {
		var part string
		part, s, _ = strings.Cut(s, "/")
		if len(b) >= 1 {
			b = append(b, '/')
		}
		b = append(b, '^')
		b = append(b, regexp.QuoteMeta(part)...)
		b = append(b, '$')
	}
	return string(b)
}
