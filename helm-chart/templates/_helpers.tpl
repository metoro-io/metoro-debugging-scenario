{{/*
Common labels
*/}}
{{- define "microservice-demo.labels" -}}
app.kubernetes.io/part-of: microservice-demo
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "microservice-demo.selectorLabels" -}}
app.kubernetes.io/name: {{ .name }}
app.kubernetes.io/part-of: microservice-demo
{{- end -}} 