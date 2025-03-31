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

{{/*
Create the init container for fault injection
*/}}
{{- define "microservice-demo.faultInjection" -}}
{{- if .faults.enabled }}
initContainers:
  - name: fault-injection
    image: busybox:1.34.1
    imagePullPolicy: Always
    command: ['sh', '-c', 'sleep 5 && echo "Injecting faults" && curl -X POST http://localhost:{{ .service.port }}/fault -H "Content-Type: application/json" -d "{\"enabled\": true, \"latency_ms\": {{ .faults.latencyMs }}, \"error_rate\": {{ .faults.errorRate }}, \"duration_sec\": {{ .faults.durationSec }}}"']
{{- end }}
{{- end -}} 