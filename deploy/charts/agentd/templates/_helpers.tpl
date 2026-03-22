{{- define "agentd.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "agentd.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := include "agentd.name" . -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "agentd.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end -}}

{{- define "agentd.labels" -}}
helm.sh/chart: {{ include "agentd.chart" . }}
app.kubernetes.io/name: {{ include "agentd.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "agentd.selectorLabels" -}}
app.kubernetes.io/name: {{ include "agentd.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "agentd.apiName" -}}
{{- printf "%s-api" (include "agentd.fullname" .) -}}
{{- end -}}

{{- define "agentd.workerName" -}}
{{- printf "%s-worker" (include "agentd.fullname" .) -}}
{{- end -}}

{{- define "agentd.nodeName" -}}
{{- printf "%s-node" (include "agentd.fullname" .) -}}
{{- end -}}

{{- define "agentd.controlPlaneEnvName" -}}
{{- printf "%s-env" (include "agentd.fullname" .) -}}
{{- end -}}

{{- define "agentd.controlPlaneSecretName" -}}
{{- printf "%s-secrets" (include "agentd.fullname" .) -}}
{{- end -}}

{{- define "agentd.image" -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) -}}
{{- end -}}

{{- define "agentd.nodeEndpoints" -}}
{{- if .Values.node.endpointsOverride -}}
{{- .Values.node.endpointsOverride -}}
{{- else if .Values.node.enabled -}}
{{- printf "%s=%s:%d" .Values.node.nodeID (include "agentd.nodeName" .) (int .Values.node.service.port) -}}
{{- end -}}
{{- end -}}

{{- define "agentd.controlPlaneEnabled" -}}
{{- if or .Values.api.enabled .Values.worker.enabled -}}true{{- end -}}
{{- end -}}

{{- define "agentd.controlPlaneSecretEnabled" -}}
{{- if and (include "agentd.controlPlaneEnabled" .) (or .Values.database.url .Values.redis.url) -}}true{{- end -}}
{{- end -}}
