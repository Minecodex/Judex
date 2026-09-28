{{- define "judex.name" -}}{{ .Release.Name | trunc 63 | trimSuffix "-" }}{{- end -}}
{{- define "judex.labels" -}}
app.kubernetes.io/name: judex
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
{{- define "judex.pgSecret" -}}{{ default (printf "%s-postgresql" .Release.Name) .Values.postgresql.existingSecret }}{{- end -}}
{{- define "judex.pgHost" -}}{{ if .Values.postgresql.embedded.enabled }}{{ .Release.Name }}-postgresql{{ else }}{{ required "External PostgreSQL host is required" .Values.postgresql.external.host }}{{ end }}{{- end -}}
{{- define "judex.storageSecret" -}}{{ if .Values.objectStorage.embedded.enabled }}judex-storage-auth{{ else }}{{ required "External S3 existingSecret is required" .Values.objectStorage.external.existingSecret }}{{ end }}{{- end -}}
{{- define "judex.databaseURL" -}}postgres://{{ .Values.postgresql.username }}:$(JUDEX_PG_PASSWORD)@{{ include "judex.pgHost" . }}:{{ .Values.postgresql.embedded.enabled | ternary "5432" (.Values.postgresql.external.port | toString) }}/{{ .Values.postgresql.database }}{{ if not .Values.postgresql.embedded.enabled }}?sslmode={{ .Values.postgresql.external.sslMode }}{{ else }}?sslmode=disable{{ end }}{{- end -}}
{{- define "judex.s3Endpoint" -}}{{ if .Values.objectStorage.embedded.enabled }}http://{{ .Release.Name }}-seaweedfs:8333{{ else }}{{ required "External S3 endpoint is required" .Values.objectStorage.external.endpoint }}{{ end }}{{- end -}}
{{- define "judex.s3Bucket" -}}{{ if .Values.objectStorage.embedded.enabled }}judex{{ else }}{{ required "External S3 bucket is required" .Values.objectStorage.external.bucket }}{{ end }}{{- end -}}
