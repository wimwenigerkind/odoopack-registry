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

{{/*
The application config as rendered into config.yaml: config, then limits,
then extraConfig on top.
*/}}
{{- define "odoopack.config" -}}
{{- $cfg := mergeOverwrite (deepCopy .Values.config) (dict "limits" (deepCopy .Values.limits)) .Values.extraConfig -}}
{{- toYaml $cfg -}}
{{- end }}

{{/*
limits.job_timeout in seconds. Sprig cannot parse a Go duration, so this
accepts the subset made of whole hours, minutes and seconds ("10m", "1h30m",
"600s") and fails on anything else.
*/}}
{{- define "odoopack.jobTimeoutSeconds" -}}
{{- $limits := (include "odoopack.config" . | fromYaml).limits | default dict -}}
{{- $d := toString (required "limits.job_timeout is required" $limits.job_timeout) -}}
{{- if not (regexMatch "^([0-9]+h)?([0-9]+m)?([0-9]+s)?$" $d) | or (eq $d "") -}}
{{- fail (printf "limits.job_timeout %q must be whole hours, minutes and seconds, such as 10m or 1h30m" $d) -}}
{{- end -}}
{{- $total := 0 -}}
{{- range $part := regexFindAll "[0-9]+[hms]" $d -1 -}}
{{- $n := regexFind "[0-9]+" $part | atoi -}}
{{- $unit := regexFind "[hms]" $part -}}
{{- $total = add $total (mul $n (get (dict "h" 3600 "m" 60 "s" 1) $unit)) -}}
{{- end -}}
{{- $total -}}
{{- end }}

{{/*
A size in the units the backend accepts (plain bytes or KiB, MiB, GiB, TiB,
case insensitive), rounded up to whole MiB. Takes a list of the size and the
key it came from, for the error message.
*/}}
{{- define "odoopack.sizeMiB" -}}
{{- $v := index . 0 -}}
{{- $key := index . 1 -}}
{{- /* YAML reads a plain byte count as a float, which toString would print in exponent notation. */ -}}
{{- $s := ternary (printf "%d" (int64 $v)) (toString $v) (kindIs "float64" $v) | trim -}}
{{- if not (regexMatch "^[0-9]+ *(?i:b|kib|mib|gib|tib)?$" $s) -}}
{{- fail (printf "%s %q must be plain bytes or KiB, MiB, GiB or TiB" $key $s) -}}
{{- end -}}
{{- $n := regexFind "^[0-9]+" $s | atoi -}}
{{- $unit := regexFind "[a-zA-Z]+$" $s | lower -}}
{{- $bytes := mul $n (get (dict "" 1 "b" 1 "kib" 1024 "mib" 1048576 "gib" 1073741824 "tib" 1099511627776) $unit) -}}
{{- div (add $bytes 1048575) 1048576 -}}
{{- end }}

{{/*
Temp space in MiB that one build needs: the clone and the zip made from it
exist side by side until the clone is removed.
*/}}
{{- define "odoopack.buildTmpMiB" -}}
{{- $limits := (include "odoopack.config" . | fromYaml).limits | default dict -}}
{{- $clone := include "odoopack.sizeMiB" (list (required "limits.clone_max_size is required" $limits.clone_max_size) "limits.clone_max_size") | atoi -}}
{{- $zip := include "odoopack.sizeMiB" (list (required "limits.zip_max_size is required" $limits.zip_max_size) "limits.zip_max_size") | atoi -}}
{{- add $clone $zip -}}
{{- end }}

{{/*
sizeLimit of the worker's /tmp in MiB: worker.count builds at once.
*/}}
{{- define "odoopack.workerTmpMiB" -}}
{{- $worker := (include "odoopack.config" . | fromYaml).worker | default dict -}}
{{- $count := int64 (required "config.worker.count is required" $worker.count) -}}
{{- if lt $count 1 -}}
{{- fail "config.worker.count must be at least 1" -}}
{{- end -}}
{{- mul $count (include "odoopack.buildTmpMiB" . | atoi) -}}
{{- end }}

{{/*
sizeLimit of the web pods' /tmp in MiB. Web clones only to rebuild a pinned
zip that is missing from storage, which is rare; room for two at once keeps a
burst of them from getting the pod evicted.
*/}}
{{- define "odoopack.webTmpMiB" -}}
{{- mul 2 (include "odoopack.buildTmpMiB" . | atoi) -}}
{{- end }}

{{/*
Container resources with ephemeral-storage filled in from the /tmp size,
unless the values set it. The limit leaves 256Mi above /tmp for logs and the
writable layer, because the kubelet counts the emptyDir against the pod's
ephemeral-storage limit. Takes a dict with resources, request and tmpMiB.
*/}}
{{- define "odoopack.resourcesWithTmp" -}}
{{- $res := deepCopy (.resources | default dict) -}}
{{- $requests := $res.requests | default dict -}}
{{- $limits := $res.limits | default dict -}}
{{- if not (hasKey $requests "ephemeral-storage") -}}
{{- $_ := set $requests "ephemeral-storage" .request -}}
{{- end -}}
{{- if not (hasKey $limits "ephemeral-storage") -}}
{{- $_ := set $limits "ephemeral-storage" (printf "%dMi" (add .tmpMiB 256)) -}}
{{- end -}}
{{- $_ := set $res "requests" $requests -}}
{{- $_ := set $res "limits" $limits -}}
{{- toYaml $res -}}
{{- end }}

{{/*
Topology spread constraints of a component, each with a labelSelector for the
component's pods unless it brings its own. Takes a dict with ctx, component
and constraints.
*/}}
{{- define "odoopack.topologySpread" -}}
{{- $labels := include "odoopack.selectorLabels" .ctx | fromYaml -}}
{{- $_ := set $labels "app.kubernetes.io/component" .component -}}
{{- $out := list -}}
{{- range .constraints -}}
{{- $out = append $out (merge (dict) . (dict "labelSelector" (dict "matchLabels" $labels))) -}}
{{- end -}}
{{- toYaml $out -}}
{{- end }}

{{/*
Refuse settings that only work in a single web process.
*/}}
{{- define "odoopack.validate" -}}
{{- $cfg := include "odoopack.config" . | fromYaml -}}
{{- $multi := or .Values.web.autoscaling.enabled (gt (int .Values.web.replicaCount) 1) -}}
{{- if and $multi (eq (toString (dig "session" "store" "" $cfg)) "memory") -}}
{{- fail "session.store memory keeps sessions in one process; use redis with more than one web replica or autoscaling" -}}
{{- end -}}
{{- if and $multi (eq (toString (dig "storage" "driver" "" $cfg)) "local") -}}
{{- fail "storage.driver local keeps files on one pod; use s3 with more than one web replica or autoscaling" -}}
{{- end -}}
{{- end }}
