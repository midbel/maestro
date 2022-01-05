package shell_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/midbel/maestro/shell"
)

const cmd = "go mod graph"

type stdio struct {
	Out bytes.Buffer
	Err bytes.Buffer
}

func (s *stdio) Reset() {
	s.Out.Reset()
	s.Err.Reset()
}

func TestShell(t *testing.T) {
	var (
		sio     stdio
		sh, err = createShell(&sio.Out, &sio.Err)
	)
	if err != nil {
		t.Fatalf("unexpected error creating shell: %s", err)
	}
	t.Run("default", func(t *testing.T) {
		executeScript(t, sh, cmd, &sio)
	})
	t.Run("redirection", func(t *testing.T) {
		executeScript(t, sh, "echo foobar > testdata/foobar.txt; if [[ -s testdata/foobar.txt ]]; then echo ok fi", &sio)
		os.Remove("testdata/foobar.txt")
	})
	t.Run("alias", func(t *testing.T) {
		sh.Alias("showgraph", cmd)
		executeScript(t, sh, "showgraph", &sio)
	})
	t.Run("assign", func(t *testing.T) {
		executeScript(t, sh, "foobar = foobar; echo ${foobar} | cut -f 1 -d 'b'", &sio)
	})
	t.Run("true-or", func(t *testing.T) {
		executeScript(t, sh, "true && echo foo", &sio)
		executeScript(t, sh, "false || echo bar", &sio)
	})
	t.Run("test", func(t *testing.T) {
		executeScript(t, sh, "if [[ -d testdata ]]; then echo ok else echo ko fi", &sio)
		executeScript(t, sh, "if [[ ! -d testdata ]]; then echo ok else echo ko fi", &sio)
	})
}

func executeScript(t *testing.T, sh *shell.Shell, script string, sio *stdio) {
	t.Helper()
	defer sio.Reset()

	err := sh.Execute(context.TODO(), script, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error executing command: %s", err)
	}
	t.Logf("length stdout: %d", sio.Out.Len())
	t.Logf("length stderr: %d", sio.Err.Len())
	if sio.Out.Len() == 0 {
		t.Errorf("stdout is empty")
	}
	if sio.Err.Len() != 0 {
		t.Errorf("stderr is not empty")
	}
}

func createShell(out, err io.Writer) (*shell.Shell, error) {
	options := []shell.ShellOption{
		shell.WithStdout(out),
		shell.WithStderr(err),
	}
	return shell.New(options...)
}
