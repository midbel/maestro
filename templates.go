package maestro

import (
	"strings"
	"text/template"
)

const helptext = `
{{if .Help -}}
{{wrap .Help}}
{{- end}}

Available commands:
{{range $k, $cs := .Commands}}
{{$k}}:
{{repeat "-" $k}}-
{{- range $cs}}
  - {{printf "%-20s %s" .Name .Short -}}
{{end}}
{{end}}

use "maestro -f {{.File}} help <command>" for more information
on one of the available command(s)
`

const cmdhelp = `
{{.Command}}

{{if .About -}}
{{.About}}
{{end}}
{{if .Desc -}}
{{.Desc}}
{{end}}
{{- if .Options -}}
Options:
{{- range .Options}}
  {{if .Short}}-{{.Short}}{{end}}{{if and .Long .Short}}, {{end}}{{if .Long}}--{{.Long}}{{end}}  {{.Help}}
{{- end}}
{{- end}}

usage: {{.Usage}}

{{if .Alias}}alias: {{join .Alias ", "}}{{end}}
{{if .Tags}}tags:  {{join .Tags ", "}}{{end}}
`

func renderTemplate(name string, ctx interface{}) (string, error) {
	t, err := template.New("template").Funcs(funcmap).Parse(name)
	if err != nil {
		return "", err
	}
	var str strings.Builder
	if err := t.Execute(&str, ctx); err != nil {
		return "", err
	}
	return str.String(), nil
}

var funcmap = template.FuncMap{
	"repeat": repeat,
	"wrap":   wrap,
	"join":   strings.Join,
}

func repeat(char string, value interface{}) string {
	var n int
	switch v := value.(type) {
	case string:
		n = len(v)
	case int:
		n = v
	default:
		return ""
	}
	return strings.Repeat(char, n)
}

func wrap(in string) (string, error) {
	return in, nil
}
