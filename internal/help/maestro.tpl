{{if .Help -}}
{{wrap .Help}}
{{- end}}

Available commands:
{{range $k, $cs := .Commands}}
{{$k}}:
{{repeat "-" $k}}-
{{- range $cs}}
  - {{printf "%-20s %s" .Name .Short -}}
{{end -}}
{{end}}

{{wrap (printf "use \"maestro -f %s help <command>\" for more information on the available command(s)" .File)}}