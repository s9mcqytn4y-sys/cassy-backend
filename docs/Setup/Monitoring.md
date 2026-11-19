# Monitoring and Alerting (P0-04, decision IN3)

Technical monitoring for one client VPS: Grafana Alloy on the VPS ships metrics, structured logs, and OTLP traces to a Grafana Cloud stack, alert rules run there, and alerts reach the operator's Telegram group and email. Store owners do not get these alerts; their business notifications come from Cassy itself.

```
api :9091/metrics --(CassyMetricsNet, internal)--+
host /proc /sys / (read-only) -------------------+- alloy --HTTPS remote write--> Grafana Cloud
/var/lib/cassy/metrics/*.prom (backup, drill) ---+                                rules -> Telegram, email
```

| File | Purpose |
|------|---------|
| `Docker/Monitoring/config.alloy` | Scrape targets, Loki JSON log stage, OTLP receiver, and remote write; mounted read-only into the `alloy` service |
| `Docker/Monitoring/alert-rules.yml` | Alert rules (Prometheus format), loaded into Grafana Cloud |
| `Docker/Monitoring/alert-rules.test.yml` | `promtool test rules` cases, run in CI |
| `Docker/Core/DockerComposeProd.yml` | `alloy` service, `CassyMetricsNet` and `CassyMonitorEgressNet` networks |

API logs are structured JSON lines containing `request_id`, `trace_id`, `store_id`, `staff_id`, `method`, `route`, `status`, and `latency_ms` (P0-03).

## 1. Alerts

| Alert | Fires when | Severity |
|-------|-----------|----------|
| `CassyMetricsMissing` | no API metrics for 10 min (VPS, Alloy or Grafana credentials down) | critical |
| `CassyAPIDown` | API scrape fails for 2 min | critical |
| `CassyHigh5xxRate` | >5% of requests are 5xx for 5 min, with at least 3 req/min | critical |
| `CassySlowRequests` | p95 latency >1 s for 10 min (`/ws` excluded) | warning |
| `CassyWorkerStalled` | reconciliation, email or outbox relay has not finished a cycle for >5 min | critical |
| `CassyEmailPermanentFailure` | an email was marked failed in the last 30 min | warning |
| `CassyDBPoolSaturated` | >90% of the 25 DB connections in use for 5 min | warning |
| `CassyBackupStale` | newest successful backup is >2 h old | critical |
| `CassyBackupMetricMissing` | no backup recorded for 2 h while host metrics arrive | critical |
| `CassyRestoreDrillStale` | no passing restore drill in 35 days | warning |
| `CassyDiskLow` / `CassyDiskCritical` | free disk <15% for 15 min / <5% for 5 min | warning / critical |
| `CassyMemoryLow` | available RAM <10% for 15 min | warning |

Every series carries `client=<CASSY_CLIENT>`, so one Grafana stack can watch several client VPSs.

## 2. Grafana Cloud setup (once per operator stack)

Steps in the Grafana Cloud UI change over time; follow the current UI if a menu name differs.

1. Create the free stack. Under the stack's Prometheus details, note the remote write URL and the instance ID (username).
2. Create two access policy tokens: one with `metrics:write`, `logs:write`, `traces:write` (for Alloy on each VPS), one with `rules:write` (for loading rules from the operator's laptop; never put it on a VPS).
3. Load the rules, from a checkout of the released tag:

   ```bash
   mimirtool rules load --address=<prometheus URL without /api/prom/push> --id=<instance id> --key=<rules:write token> Docker/Monitoring/alert-rules.yml
   ```

4. Contact points (Alerting -> Contact points, on the stack's Mimir/Prometheus Alertmanager, since these are data-source-managed rules): a Telegram contact point (bot token from @BotFather, chat id of the operator group) and an email contact point.
5. Notification policy: `severity=critical` -> Telegram and email; `severity=warning` -> Telegram. Group by `client, alertname`.
6. Uptime check (needs the production domain): Testing & synthetics -> Synthetic Monitoring -> HTTP check on `https://<domain>/health/ready` from at least two probe locations, with alerting enabled. This catches an outage even when the whole VPS, and therefore Alloy, is gone.
7. Check the free-tier active series quota in the stack's usage page after the first day of traffic.

## 3. VPS setup (per client)

1. `mkdir -p /var/lib/cassy/metrics && chown cassy /var/lib/cassy/metrics` before the first `docker compose up` (otherwise Docker creates it owned by root and the backup cron cannot write its metric, which `CassyBackupMetricMissing` will report).
2. Add to the production `.env` (values from the password manager):

   ```bash
   CASSY_CLIENT=<client slug>
   GRAFANA_CLOUD_PROM_URL=<remote write URL>
   GRAFANA_CLOUD_PROM_USER=<instance id>
   GRAFANA_CLOUD_LOKI_URL=<loki push URL>
   GRAFANA_CLOUD_LOKI_USER=<loki instance id>
   GRAFANA_CLOUD_OTLP_URL=<otlp endpoint URL>
   GRAFANA_CLOUD_OTLP_USER=<tempo/otlp instance id>
   GRAFANA_CLOUD_API_KEY=<metrics/logs/traces write token>
   ```

3. `docker compose -f Docker/Core/DockerComposeProd.yml up -d alloy`
4. Verify in Grafana Explore: `up{client="<client slug>"}` shows `job="cassy-api"` and `job="integrations/unix"` at 1, and `cassy_backup_last_success_timestamp_seconds{client="<client slug>"}` appears after the next hourly backup.
5. Test the route once: temporarily stop the API (`docker compose stop api` during a maintenance window) and confirm `CassyAPIDown` reaches Telegram within ~3 minutes, then start it again.

## 4. Notes

- The API listens for metrics on `:9091` inside its container. No host port is published and Nginx does not route it; containers on `CassyEdgeNet` (nginx, web) cannot reach `:9091` except on `CassyMetricsNet`, and the metrics carry no ids, tokens or customer data (P0-03).
- The root filesystem is mounted with `rslave` so new host mounts show up; this needs a shared `/` mount, which is the Ubuntu/systemd default (Docker Desktop does not support it).
- A worker's last-success gauge appears after its first completed cycle, so a worker that fails from its very first cycle after a restart is not alerted by `CassyWorkerStalled`; it only shows in the API logs (`[EMAIL WORKER]`, `[RECONCILIATION WORKER]` lines).
