{{/*
Auth subchart helpers
*/}}

{{- define "auth.namespace" -}}auth-system{{- end }}

{{- define "auth.labels" -}}
app.kubernetes.io/part-of: devplatform
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
