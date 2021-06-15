{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "brigade-prometheus.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "brigade-prometheus.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "brigade-prometheus.exporter.fullname" -}}
{{ include "brigade-prometheus.fullname" . | printf "%s-apiserver" }}
{{- end -}}

{{- define "brigade-prometheus.prometheus.fullname" -}}
{{ include "brigade-prometheus.fullname" . | printf "%s-scheduler" }}
{{- end -}}

{{- define "brigade-prometheus.grafana.fullname" -}}
{{ include "brigade-prometheus.fullname" . | printf "%s-observer" }}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "brigade-prometheus.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "brigade-prometheus.labels" -}}
helm.sh/chart: {{ include "brigade-prometheus.chart" . }}
{{ include "brigade-prometheus.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "brigade-prometheus.selectorLabels" -}}
app.kubernetes.io/name: {{ include "brigade-prometheus.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "brigade-prometheus.exporter.labels" -}}
app.kubernetes.io/component: exporter
{{- end -}}

{{- define "brigade-prometheus.prometheus.labels" -}}
app.kubernetes.io/component: prometheus
{{- end -}}

{{- define "brigade-prometheus.grafana.labels" -}}
app.kubernetes.io/component: grafana
{{- end -}}

{{- define "call-nested" }}
{{- $dot := index . 0 }}
{{- $subchart := index . 1 }}
{{- $template := index . 2 }}
{{- include $template (dict "Chart" (dict "Name" $subchart) "Values" (index $dot.Values $subchart) "Release" $dot.Release "Capabilities" $dot.Capabilities) }}
{{- end }}
