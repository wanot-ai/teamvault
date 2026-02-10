{{/*
Expand the name of the chart.
*/}}
{{- define "teamvault.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this
(by the DNS naming spec). If release name contains chart name it will be used as
a full name.
*/}}
{{- define "teamvault.fullname" -}}
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
{{- define "teamvault.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "teamvault.labels" -}}
helm.sh/chart: {{ include "teamvault.chart" . }}
{{ include "teamvault.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: teamvault
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "teamvault.selectorLabels" -}}
app.kubernetes.io/name: {{ include "teamvault.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use.
*/}}
{{- define "teamvault.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "teamvault.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Return the image name.
*/}}
{{- define "teamvault.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}

{{/*
Return the secret name for TeamVault credentials.
*/}}
{{- define "teamvault.secretName" -}}
{{- if .Values.secrets.existingSecret }}
{{- .Values.secrets.existingSecret }}
{{- else }}
{{- include "teamvault.fullname" . }}
{{- end }}
{{- end }}

{{/*
Return the database URL.
If database.existingSecret is set, it is expected to contain the full URL.
Otherwise, construct from individual fields.
*/}}
{{- define "teamvault.databaseURL" -}}
{{- if .Values.config.databaseURL }}
{{- .Values.config.databaseURL }}
{{- else if .Values.postgresql.enabled }}
{{- printf "postgres://%s:%s@%s-postgresql:5432/%s?sslmode=disable" .Values.postgresql.auth.username .Values.postgresql.auth.password .Release.Name .Values.postgresql.auth.database }}
{{- else }}
{{- printf "postgres://%s:%s@%s:%v/%s?sslmode=%s" .Values.database.user .Values.database.password .Values.database.host (.Values.database.port | toString) .Values.database.name .Values.database.sslMode }}
{{- end }}
{{- end }}

{{/*
Return the TLS secret name.
*/}}
{{- define "teamvault.tlsSecretName" -}}
{{- if .Values.tls.existingSecret }}
{{- .Values.tls.existingSecret }}
{{- else }}
{{- printf "%s-tls" (include "teamvault.fullname" .) }}
{{- end }}
{{- end }}
