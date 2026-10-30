//go:build linux

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

const testNameSuffix = "/strace"

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
	t *testing.T,
	testName string,
	testFunc func(t *testing.T, tempDir string),
	straceArgs ...string,
) {
	t.Helper()

	if err := initOnce(); err != nil {
		t.Fatalf("stracetest: failed to init: %v", err)
	}

	if straceExePath == "" {
		t.Run(testName, func(t *testing.T) { t.Skip("stracetest: strace not found in PATH") })
		return
	}

	if tempDir := os.Getenv(tempDirEnvKey); tempDir != "" {
		// is already under strace
		t.Run(testName+testNameSuffix, func(t *testing.T) { testFunc(t, tempDir) })
		return
	}

	t.Run(testName, func(t *testing.T) {
		t.Helper()

		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("stracetest: failed to create pipe: %v", err)
		}
		var wg sync.WaitGroup
		wg.Go(func() {
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				t.Log(scanner.Text())
			}
			_ = scanner.Err()
			io.Copy(io.Discard, r)
		})
		t.Cleanup(func() {
			w.Close()
			wg.Wait()
			r.Close()
		})

		if err := buildTestCmd(t, straceArgs, w).Run(); err != nil {
			t.Fatalf("stracetest: failed to run test: %v", err)
		}
	})
}

func buildTestCmd(t *testing.T, straceArgs []string, straceOutputFile *os.File) *exec.Cmd {
	t.Helper()

	tempDir := t.TempDir()

	var args []string
	for _, straceArg := range straceArgs {
		args = append(args, strings.ReplaceAll(straceArg, "{{TempDir}}", tempDir))
	}
	args = append(args,
		"--follow-forks",
		"--output=/dev/fd/3",
	)
	args = append(args, "--", testExePath)
	args = append(args, os.Args[1:]...)
	// override existing -test.run and -test.count
	args = append(args,
		"-test.run", buildTestRunPattern(t.Name()+testNameSuffix),
		"-test.count", "1",
	)

	cmd := exec.CommandContext(t.Context(), straceExePath, args...)
	cmd.Env = append(os.Environ(), tempDirEnvKey+"="+tempDir)
	cmd.ExtraFiles = []*os.File{straceOutputFile}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd
}

func buildTestRunPattern(testName string) string {
	var builder strings.Builder
	for s := testName; s != ""; {
		var part string
		part, s, _ = strings.Cut(s, "/")
		if builder.Len() >= 1 {
			builder.WriteByte('/')
		}
		builder.WriteByte('^')
		builder.WriteString(regexp.QuoteMeta(part))
		builder.WriteByte('$')
	}
	return builder.String()
}
