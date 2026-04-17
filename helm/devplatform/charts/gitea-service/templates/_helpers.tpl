{{- define "gitea-service.name" -}}gitea-service{{- end }}
{{- define "gitea-service.fullname" -}}gitea-service{{- end }}
{{- define "gitea-service.namespace" -}}dev-platform{{- end }}
{{- define "gitea-service.labels" -}}
app: gitea-service
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: devplatform
{{- end }}
{{- define "gitea-service.selectorLabels" -}}
app: gitea-service
{{- end }}
