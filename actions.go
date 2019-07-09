package maestro

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"
)

var usage = `
{{- if .Desc}}
	{{- .Desc}}
{{else}}
	{{- .Help}}

{{ end -}}
Properties:

- shell  : {{.Shell}}
- workdir: {{.Workdir}}
- stdout : {{.Stdout}}
- stderr : {{.Stderr}}
- env    : {{.Env}}
- ignore : {{.Ignore}}
- retry  : {{.Retry}}
- delay  : {{.Delay}}
- timeout: {{.Timeout}}

{{if .Locals -}}
Local variables:

{{range $k, $vs := .Locals}}
	{{- printf "- %-12s: %s" $k (join $vs " ")}}
{{end}}
{{- end}}
{{- if .Globals}}
Environment Variables:

{{range $k, $v := .Globals}}
	{{- printf "- %-12s: %s" $k $v}}
{{end}}
{{- end}}
{{if .Dependencies}}
Dependencies:
{{range .Dependencies}}
- {{ . -}}
{{end}}
{{end}}
`

type Action struct {
	Name string
	Help string
	Desc string
	Tags []string

	Dependencies []string

	Script string
	Shell  string // bash, sh, ksh, python,...

	Hazard  bool
	Inline  bool
	Env     bool
	Ignore  bool
	Retry   int64
	Delay   time.Duration
	Timeout time.Duration
	Workdir string
	Stdout  string
	Stderr  string
	// remaining arguments from command line
	Args []string

	// environment variables + locals variables
	locals  map[string][]string
	globals map[string]string
}

func (a Action) Usage() error {
	fs := template.FuncMap{
		"join": strings.Join,
	}
	t, err := template.New("usage").Funcs(fs).Parse(strings.TrimSpace(usage))
	if err != nil {
		return err
	}
	d := struct {
		Action
		Locals  map[string][]string
		Globals map[string]string
	}{
		Action:  a,
		Locals:  a.locals,
		Globals: a.globals,
	}
	return t.Execute(os.Stdout, d)
}

func (a Action) String() string {
	s, err := a.prepareScript()
	if err != nil {
		s = err.Error()
	}
	return s
}

func (a Action) Execute() error {
	if a.Script == "" {
		return nil
	}
	script, err := a.prepareScript()
	if err != nil {
		return err
	}
	args := ParseShell(a.Shell)
	if len(args) == 0 {
		return fmt.Errorf("%s: fail to parse shell", a.Shell)
	}

	for i := int64(0); i < a.Retry; i++ {
		if err = a.executeScript(args, script); err == nil {
			break
		}
	}
	if a.Ignore && err != nil {
		err = nil
	}
	return err
}

func (a Action) executeScript(args []string, script string) error {
	if a.Delay > 0 {
		time.Sleep(a.Delay)
	}
	if a.Inline {
		args = append(args, script)
	}
	cmd := exec.Command(args[0], args[1:]...)
	// cmd := exec.Command(args[0], append(args[1:], script)...)
	if i, err := os.Stat(a.Workdir); err == nil && i.IsDir() {
		cmd.Dir = a.Workdir
	} else {
		if a.Workdir != "" {
			return fmt.Errorf("%s: not a directory", a.Workdir)
		}
	}

	if !a.Inline {
		cmd.Stdin = strings.NewReader(script)
	}
	openFD := func(n string, w io.Writer) (io.Writer, error) {
		if n == "" {
			return w, nil
		} else if n == "discard" || n == "-" {
			return ioutil.Discard, nil
		} else {
			return os.OpenFile(n+".err", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		}
	}
	var err error
	if cmd.Stdout, err = openFD(a.Stdout, os.Stdout); err != nil {
		return err
	}
	if cmd.Stderr, err = openFD(a.Stderr, os.Stderr); err != nil {
		return err
	}

	if a.Env {
		cmd.Env = append(cmd.Env, os.Environ()...)
		for k, v := range a.globals {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return cmd.Run()
}

func (a Action) prepareScript() (string, error) {
	var (
		b strings.Builder
		n int
	)

	script := []byte(a.Script)
	for {
		k, nn := utf8.DecodeRune(script[n:])
		if k == utf8.RuneError {
			if nn == 0 {
				break
			} else {
				return "", fmt.Errorf("invalid character found in script!!!")
			}
		}
		n += nn
		if k == percent {
			x := n
			for k != rparen {
				k, nn = utf8.DecodeRune(script[x:])
				x += nn
			}
			str, err := a.expandVariable(string(script[n:x]))
			if err != nil {
				return str, err
			}
			b.WriteString(str)
			n = x
		} else {
			b.WriteRune(k)
		}
	}
	return b.String(), nil
}

func (a Action) expandVariable(str string) (string, error) {
	str = strings.Trim(str, "()")
	if len(str) == 0 {
		return "", fmt.Errorf("script: invalid syntax")
	}
	if str == "TARGET" {
		str = a.Name
	} else if str == "#" {
		str = strconv.Itoa(len(a.Args))
	} else if str == "@" {
		str = strings.Join(a.Args, " ")
	} else if s, ok := a.locals[str]; ok {
		str = strings.Join(s, " ")
	} else {
		switch str {
		case "shell":
			str = a.Shell
		case "workdir":
			str = a.Workdir
		case "stdout":
			str = a.Stdout
		case "stderr":
			str = a.Stderr
		case "env":
			str = strconv.FormatBool(a.Env)
		case "ignore":
			str = strconv.FormatBool(a.Ignore)
		case "retry":
			str = strconv.FormatInt(a.Retry, 10)
		case "delay":
			str = a.Delay.String()
		case "timeout":
			str = a.Timeout.String()
		default:
			x, err := strconv.ParseInt(str, 10, 64)
			if err == nil && (x >= 0 && int(x) < len(a.Args)) {
				str = a.Args[x]
			} else {
				return "", fmt.Errorf("%s: variable not defined", str)
			}
		}
	}
	return str, nil
}
