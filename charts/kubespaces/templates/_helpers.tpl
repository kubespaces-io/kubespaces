{{/* Chart name */}}
{{- define "kubespaces.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/* Fully qualified release name */}}
{{- define "kubespaces.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/* Common labels */}}
{{- define "kubespaces.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/name: {{ include "kubespaces.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/* Selector labels for a component; call with (dict "ctx" . "component" "api") */}}
{{- define "kubespaces.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kubespaces.name" .ctx }}
app.kubernetes.io/instance: {{ .ctx.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{/* Image reference; call with (dict "ctx" . "image" .Values.api.image) */}}
{{- define "kubespaces.image" -}}
{{- printf "%s:%s" .image.repository (default .ctx.Chart.AppVersion .image.tag) -}}
{{- end -}}

{{/* ---------- Database ---------- */}}

{{- define "kubespaces.db.host" -}}
{{- if .Values.postgresql.enabled -}}
{{ include "kubespaces.fullname" . }}-postgres
{{- else -}}
{{ required "externalDatabase.host is required when postgresql.enabled=false" .Values.externalDatabase.host }}
{{- end -}}
{{- end -}}

{{- define "kubespaces.db.port" -}}
{{- if .Values.postgresql.enabled -}}5432{{- else -}}{{ .Values.externalDatabase.port }}{{- end -}}
{{- end -}}

{{- define "kubespaces.db.name" -}}
{{- if .Values.postgresql.enabled -}}{{ .Values.postgresql.database }}{{- else -}}{{ .Values.externalDatabase.database }}{{- end -}}
{{- end -}}

{{- define "kubespaces.db.user" -}}
{{- if .Values.postgresql.enabled -}}{{ .Values.postgresql.username }}{{- else -}}{{ .Values.externalDatabase.username }}{{- end -}}
{{- end -}}

{{/* Name of the Secret holding key "password" for the database */}}
{{- define "kubespaces.db.secretName" -}}
{{- if .Values.postgresql.enabled -}}
{{- default (printf "%s-postgres" (include "kubespaces.fullname" .)) .Values.postgresql.existingSecret -}}
{{- else -}}
{{ required "externalDatabase.existingSecret is required when postgresql.enabled=false" .Values.externalDatabase.existingSecret }}
{{- end -}}
{{- end -}}

{{/* ---------- OIDC ---------- */}}

{{- define "kubespaces.oidc.issuerUrl" -}}
{{- if .Values.oidc.issuerUrl -}}
{{ .Values.oidc.issuerUrl }}
{{- else if .Values.keycloak.enabled -}}
{{- /* built-in Keycloak serves under --http-relative-path /auth */ -}}
http://{{ include "kubespaces.fullname" . }}-keycloak:{{ .Values.keycloak.service.port }}/auth/realms/{{ .Values.keycloak.realm }}
{{- else -}}
{{ required "oidc.issuerUrl is required when keycloak.enabled=false" .Values.oidc.issuerUrl }}
{{- end -}}
{{- end -}}

{{/* ---------- Secret auto-generation ----------
Generate-once pattern: reuse the existing Secret value on upgrade so
auto-generated passwords are stable across helm upgrades. */}}
{{- define "kubespaces.generatedPassword" -}}
{{- $ns := .ctx.Release.Namespace -}}
{{- $existing := lookup "v1" "Secret" $ns .secretName -}}
{{- if $existing -}}
{{- index $existing.data .key -}}
{{- else -}}
{{- randAlphaNum 32 | b64enc -}}
{{- end -}}
{{- end -}}
