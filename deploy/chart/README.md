# bikeshare-city Helm chart

One templated per-city tracker stack (namespace + host-Postgres Service/Endpoints +
dockscan ingester + web Deployment/Service/Ingress). A city is **pure config** —
`values/<city>.yaml` — so adding a 5th is filling in a values file, not copying YAML.
This replaces the old hand-written `deploy/k8s/<city>/` manifests, which the chart
reproduces (verified via `kubectl diff` — only the explicit defaults differ).

Render / apply an existing city:

```sh
helm template cabi deploy/chart -f deploy/chart/values/cabi.yaml | kubectl apply -f -
# (or: helm upgrade --install cabi deploy/chart -f deploy/chart/values/cabi.yaml)
```

## Adding a new city (full runbook)

The chart renders the in-cluster resources; these one-time, out-of-band pieces still
need creating first (same as the existing cities — they hold secrets/data, not templated):

1. **Geography** — write `scripts/build_<city>.py`, produce `configs/<city>/{neighborhoods.json,
   neighborhoods.meta.json}` (validate against the live feed). Generate `configs/<city>/og.png`.
2. **Host DB** — `docker run` a `timescale/timescaledb` container on a free port, create the
   `<city>_ro` + `monitoring` roles (see how cabi-postgres was set up).
3. **Umami** — POST `/api/websites` to analytics.kardol.us → website id.
4. **Cluster prereqs** (per namespace `<name>`):
   - `kubectl create ns <name>` + copy the `ghcr-pull` imagePullSecret.
   - ConfigMaps from the config files: `<name>-neighborhoods` (neighborhoods.json),
     `<name>-neighborhoods-meta` (neighborhoods.meta.json), `<name>-og` (og.png).
   - Secrets `<name>-db` (rw) and `<name>-web-db` (ro) with the `DATABASE_URL`.
5. **Values + deploy** — write `values/<city>.yaml`, then `helm template … | kubectl apply -f -`.
6. **Migrations** — apply `citibike-web/deploy/sql/{01,02}*.sql` (sed the role to `<city>_ro`).
7. **Edge** — add `<domain>` to the Cloudflare tunnel config + a proxied CNAME.
8. **Monitoring** — add `https://<domain>` to `monitoring` values `probes.http2xx` + the
   `ServiceProbeDown` host regex; add `<name>` to `citibike.namespaces`; `make deploy`.

## Values

See `values.yaml` for the full schema (resource names derive from `name`). Each city sets:
feed URLs, `cityId/brand/domain/timezone`, `hasEbikes`, `defaultScope`, `neighborhoodLabel`,
`areaOverrides` (JSON), `umamiId`, `description`, `pgPort`, and the two image tags.
