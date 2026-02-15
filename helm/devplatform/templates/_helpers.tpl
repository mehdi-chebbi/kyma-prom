{{/*
Shared template helpers for the DevPlatform umbrella chart.
*/}}

{{/*
Expand the name of the chart.
*/}}
{{- define "devplatform.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "devplatform.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "devplatform.labels" -}}
helm.sh/chart: {{ include "devplatform.name" . }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: devplatform
{{- end }}

{{/*
Selector labels
*/}}
{{- define "devplatform.selectorLabels" -}}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Namespace helpers
*/}}
{{- define "devplatform.namespace.devPlatform" -}}
dev-platform
{{- end }}

{{- define "devplatform.namespace.auth" -}}
auth-system
{{- end }}

{{- define "devplatform.namespace.codeserverInstances" -}}
codeserver-instances
{{- end }}
