{{- define "codeserver.name" -}}codeserver-service{{- end }}
{{- define "codeserver.fullname" -}}codeserver-service{{- end }}
{{- define "codeserver.namespace" -}}dev-platform{{- end }}
{{- define "codeserver.labels" -}}
app: codeserver-service
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: devplatform
{{- end }}
{{- define "codeserver.selectorLabels" -}}
app: codeserver-service
{{- end }}
