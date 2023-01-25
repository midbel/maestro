{{.Command}}{{if .About }}: {{.About}}{{end}}

{{if .Desc -}}{{wrap .Desc}}
{{end}}

{{- with .Options}}
Options:
{{range . }}
  {{if .Short}}-{{.Short}}{{end}}{{if and .Long .Short}}, {{end}}{{if .Long}}--{{.Long}}{{end}}{{if .Help}}  {{.Help}}{{end}}
{{- end}}
{{end}}
usage: {{.Usage}}
{{if .Alias}}alias: {{join .Alias ", "}}
{{end -}}
{{if .Tags}}tags:  {{join .Tags ", "}}
{{end -}}