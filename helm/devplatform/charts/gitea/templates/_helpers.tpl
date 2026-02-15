{{- define "gitea.name" -}}gitea{{- end }}
{{- define "gitea.fullname" -}}gitea{{- end }}
{{- define "gitea.namespace" -}}dev-platform{{- end }}
{{- define "gitea.labels" -}}
app: gitea
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: devplatform
{{- end }}
{{- define "gitea.selectorLabels" -}}
app: gitea
{{- end }}
