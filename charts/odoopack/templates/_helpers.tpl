{{/*
Expand the name of the chart.
*/}}
{{- define "odoopack.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "odoopack.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "odoopack.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "odoopack.labels" -}}
helm.sh/chart: {{ include "odoopack.chart" . }}
{{ include "odoopack.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "odoopack.selectorLabels" -}}
app.kubernetes.io/name: {{ include "odoopack.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "odoopack.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "odoopack.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Backend image reference (web, worker, migration).
*/}}
{{- define "odoopack.backendImage" -}}
{{- printf "%s/%s:%s" .Values.image.registry .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) -}}
{{- end }}

{{/*
Frontend image reference.
*/}}
{{- define "odoopack.frontendImage" -}}
{{- printf "%s/%s:%s" .Values.frontend.image.registry .Values.frontend.image.repository (.Values.frontend.image.tag | default .Chart.AppVersion) -}}
{{- end }}

{{/*
Name of the Secret providing env vars.
*/}}
{{- define "odoopack.secretName" -}}
{{- if .Values.existingSecret -}}
{{- .Values.existingSecret -}}
{{- else -}}
{{- printf "%s-env" (include "odoopack.fullname" .) -}}
{{- end }}
{{- end }}

{{/*
Whether any Secret is wired (rendered or existing).
*/}}
{{- define "odoopack.hasSecret" -}}
{{- if or .Values.existingSecret (gt (len .Values.secrets) 0) -}}true{{- end }}
{{- end }}
