{{/*
OpenLDAP subchart helpers
*/}}

{{- define "openldap.name" -}}
openldap
{{- end }}

{{- define "openldap.fullname" -}}
{{ include "openldap.name" . }}
{{- end }}

{{- define "openldap.namespace" -}}
dev-platform
{{- end }}

{{- define "openldap.labels" -}}
app: {{ include "openldap.name" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: devplatform
{{- end }}

{{- define "openldap.selectorLabels" -}}
app: {{ include "openldap.name" . }}
{{- end }}
