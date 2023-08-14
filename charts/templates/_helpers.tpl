{{- define "discord-tech-news-notifier.name" -}}
{{ printf "%s-%s" "discord-tech-news-notifier" .Chart.AppVersion | replace "." "-" }}
{{- end -}}

{{- define "discord-tech-news-notifier.webhookUrl" -}}
{{ required "You must provide the webhook url; .Values.discordWebhook.url" .Values.discordWebhook.url | b64enc }}
{{- end -}}

{{- define "discord-tech-news-notifier.volumeClaimName" -}}
{{ "discord-tech-news-notifier" }}
{{- end -}}
