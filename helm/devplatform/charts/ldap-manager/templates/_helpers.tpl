{{- define "ldap-manager.name" -}}ldap-manager{{- end }}
{{- define "ldap-manager.fullname" -}}ldap-manager{{- end }}
{{- define "ldap-manager.namespace" -}}dev-platform{{- end }}
{{- define "ldap-manager.labels" -}}
app: ldap-manager
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: devplatform
{{- end }}
{{- define "ldap-manager.selectorLabels" -}}
app: ldap-manager
{{- end }}
