# Grafana Dashboards

Place Grafana dashboard JSON exports here.

Planned dashboards:
- `opcfs-overview.json` — request rate, error rate, p99 latency
- `opcfs-storage.json`  — upload throughput, chunk assembly lag, backend errors
- `opcfs-db.json`       — query latency, connection pool utilisation

Import via the Grafana UI (`Dashboards → Import`) or provision automatically
by mounting this directory into the Grafana container at
`/etc/grafana/provisioning/dashboards/`.
