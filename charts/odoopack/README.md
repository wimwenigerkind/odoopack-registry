# odoopack Helm chart

Deploys the odoopack registry: the web API, the background worker and the frontend. Database migrations run in an init container of the web pods.

## Install

The chart is published as an OCI artifact on GHCR.

```sh
helm install odoopack oci://ghcr.io/wimwenigerkind/charts/odoopack \
  --version 0.1.1 \
  --namespace odoopack --create-namespace \
  --set config.base_url=https://packages.example.com \
  --set config.storage.s3.endpoint=https://s3.example.com \
  --set config.storage.s3.bucket=odoopack \
  --set config.storage.s3.access_key_id=KEY \
  --set secrets.DATABASE_DSN='host=postgres port=5432 user=odoopack password=... dbname=odoopack sslmode=disable' \
  --set secrets.STORAGE_S3_SECRET_ACCESS_KEY=SECRET \
  --set secrets.REDIS_URL=redis://redis:6379/0
```

## Prerequisites

- An external PostgreSQL database (pass its DSN via `secrets.DATABASE_DSN` or `existingSecret`).
- S3-compatible object storage, shared by the web and worker pods.
- An external Redis for sessions and login state (pass its URL via `secrets.REDIS_URL` or `existingSecret`). The `memory` store is for local development only and the web pods refuse it unless `dev_mode` is `true`.

## Configuration

Non-secret settings live under `config` and are rendered into a `config.yaml` that the pods read. Sensitive values are passed as environment variables through a Secret.

| Key | Default | Description |
| --- | --- | --- |
| `image.repository` | `wimwenigerkind/odoopack-registry-backend` | Backend image (web, worker, migrate) |
| `image.tag` | chart appVersion | Backend image tag |
| `frontend.image.repository` | `wimwenigerkind/odoopack-registry-frontend` | Frontend image |
| `frontend.enabled` | `true` | Deploy the frontend |
| `frontend.port` | `8080` | Port the unprivileged nginx listens on; the frontend Service stays on port 80 |
| `frontend.resources` | see [Resources](#resources) | Resources of the frontend container |
| `frontend.tmpSizeLimit` | `64Mi` | `sizeLimit` of the frontend's `/tmp` emptyDir |
| `frontend.podSecurityContext` | see [Security](#security) | Pod securityContext of the frontend pods (uid `101`) |
| `frontend.securityContext` | see [Security](#security) | Container securityContext of the frontend |
| `web.replicaCount` | `1` | Web replicas |
| `web.autoscaling.enabled` | `false` | HPA for the web deployment |
| `web.resources` | see [Resources](#resources) | Resources of the web container |
| `web.topologySpreadConstraints` | one soft constraint by hostname | Spread of the web pods; each entry gets a labelSelector for the web pods unless it has one. `[]` turns it off |
| `web.podDisruptionBudget.enabled` | `true` | PDB for the web pods |
| `web.podDisruptionBudget.maxUnavailable` | `1` | Web pods a voluntary disruption may take down at once |
| `worker.replicaCount` | `1` | Worker replicas |
| `worker.healthPort` | `6970` | Worker health endpoint port |
| `worker.resources` | see [Resources](#resources) | Resources of the worker container |
| `worker.podDisruptionBudget.enabled` | `true` | PDB for the worker pods |
| `worker.podDisruptionBudget.maxUnavailable` | `1` | Worker pods a voluntary disruption may take down at once |
| `migration.enabled` | `true` | Run migrations in the `migrate` init container of the web pods |
| `migration.resources` | see [Resources](#resources) | Resources of the `migrate` init container |
| `serviceAccount.automount` | `false` | Mount a ServiceAccount token; set on the ServiceAccount and on every pod. No pod uses the Kubernetes API |
| `podSecurityContext` | see [Security](#security) | Pod securityContext of the web and worker pods (uid `65532`) |
| `securityContext` | see [Security](#security) | Container securityContext of web, worker and `migrate` |
| `config.base_url` | `http://localhost` | Public base URL, must be set |
| `config.instance.mode` | `public` | `public` or `private` |
| `config.storage.driver` | `s3` | Storage driver |
| `config.storage.s3.*` | | S3 endpoint, region, bucket, access_key_id, flags |
| `config.session.store` | `redis` | `redis`, or `memory` together with `dev_mode: true` |
| `config.worker.count` | `2` | Builds one worker pod runs at once; sizes the worker's `/tmp`, see [Temp space](#temp-space) |
| `config.auth.cookie_secure` | unset | Secure session cookies; unset, true when `base_url` is https |
| `extraConfig` | `{}` | Deep-merged over `config`, use for auth providers |
| `secrets` | `{}` | Secret env vars rendered into a Secret |
| `existingSecret` | `""` | Use a pre-existing Secret instead of `secrets` |
| `extraEnv` | `[]` | Extra env vars for all backend pods |
| `extraEnvFrom` | `[]` | Extra envFrom sources |
| `ingress.enabled` | `false` | Path-split ingress (API to web, rest to frontend) |
| `ingress.backendPaths` | `/api`, `/registry`, `/auth`, `/integrations`, `/webhooks` | Path prefixes routed to the web service |
| `networkPolicy.*` | disabled | See [Network policies](#network-policies) |

### Database migrations

Every web pod runs `migrate` (the same backend image, Secret and `config.yaml`) as an init container before the web container starts. Pods that start together, after a scale up or during a rolling update, serialize on a Postgres advisory lock: the first one migrates, the others wait and then find nothing left to do. The web container itself never migrates (`AUTO_MIGRATE=false`). This is deliberately not a Helm hook: a pre-install hook runs before the Secret and ConfigMap of the same release exist, so it cannot start on a fresh install.

During a rolling update the old pods keep serving while a new pod migrates, so a release only adds to the schema in ways the previous release can live with.

The worker does not migrate. Until a migration has created the schema it waits, checking with a backoff of up to 30 seconds, and starts taking jobs afterwards.

Set `migration.enabled: false` only if you run `migrate` yourself, against the same database and with the same config and Secret, before every upgrade.

### Images

Both images run as non-root users and work with a read-only root filesystem.

The backend image (web, worker, migrate) runs as uid and gid `65532`. It writes only to:

- `/tmp` (`os.TempDir()`, override with `TMPDIR`): the worker's clones and zips, up to `limits.clone_max_size` plus `limits.zip_max_size` per running job; web rebuilds missing pinned zips and may buffer request bodies there. The chart mounts an `emptyDir` here, see [Temp space](#temp-space).
- `storage.local.root` (default `./data/storage`, so `/data/data/storage`): only with the `local` storage driver. Mount a volume at that path; the directory in the image belongs to uid `65532`.

`HOME` is `/tmp` and git reads neither a system nor a global config (`GIT_CONFIG_NOSYSTEM=1`, `GIT_CONFIG_GLOBAL=/dev/null`), so git needs no writable home directory. `/data/config.yaml` is read only.

The frontend image is the unprivileged nginx (uid `101`) listening on `8080`. It writes its pid file and temp files to `/tmp`, where the chart mounts an `emptyDir` of `frontend.tmpSizeLimit`.

### Security

Every pod runs with the restricted Pod Security Standard in mind:

```yaml
podSecurityContext:            # web and worker; frontend.podSecurityContext uses 101
  runAsNonRoot: true
  runAsUser: 65532
  runAsGroup: 65532
  seccompProfile:
    type: RuntimeDefault
securityContext:               # web, worker, migrate; frontend.securityContext is the same
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
```

No volume of the chart needs `fsGroup`: the `/tmp` emptyDirs are writable by every user and the ConfigMap is read only. Add `fsGroup: 65532` to `podSecurityContext` if you patch in a volume that the backend writes to, such as one for the `local` storage driver. Helm merges an override into these blocks key by key; set a key to `null` to drop it.

No pod gets a ServiceAccount token (`serviceAccount.automount: false`).

The chart refuses to render the `memory` session store or the `local` storage driver together with more than one web replica or autoscaling: both keep their state in a single pod.

### Health endpoints

`/healthz` and `/readyz` of web and worker are for the kubelet only. The ingress does not route them (they were in `ingress.backendPaths` before; remove them there if you copied that list into your values). `/readyz` gives its checks 3 seconds, so the readiness probes wait 5 seconds (`timeoutSeconds`) and the liveness probes 3.

### Resources

Every container has requests and limits:

| Container | Requests | Limits |
| --- | --- | --- |
| `web` | `100m` CPU, `128Mi` memory, `256Mi` ephemeral-storage | `1` CPU, `512Mi` memory, `/tmp` + `256Mi` ephemeral-storage |
| `worker` | `100m` CPU, `256Mi` memory, `/tmp` ephemeral-storage | `1` CPU, `1Gi` memory, `/tmp` + `256Mi` ephemeral-storage |
| `migrate` | `50m` CPU, `64Mi` memory | `500m` CPU, `256Mi` memory |
| `frontend` | `10m` CPU, `32Mi` memory, `16Mi` ephemeral-storage | `200m` CPU, `128Mi` memory, `128Mi` ephemeral-storage |

The `ephemeral-storage` of web and worker follows from the size of their `/tmp` (below) unless `web.resources` or `worker.resources` set it: the kubelet counts an emptyDir against the pod's ephemeral-storage limit, so the limit is `/tmp` plus `256Mi` for logs and the container layer. The worker requests all of its `/tmp`, so it lands only on a node with that much disk; web requests `256Mi`, because it rarely clones. Overrides merge into the defaults key by key; set a key to `null` to drop it, for example `web.resources.limits.cpu: null` for no CPU limit.

### Temp space

Web and worker get an `emptyDir` at `/tmp` whose `sizeLimit` the chart computes from the limits instead of taking a value of its own, so the two cannot drift apart. One build needs `limits.clone_max_size` plus `limits.zip_max_size`, because the zip is written next to the clone:

| Pod | `/tmp` sizeLimit | Default |
| --- | --- | --- |
| worker | `config.worker.count` times one build | 2 x (1GiB + 100MiB) = `2248Mi` |
| web | two builds, for on-demand rebuilds of pinned zips missing from storage | `2248Mi` |

To change it, change `limits.clone_max_size`, `limits.zip_max_size` or `config.worker.count` (also through `extraConfig`); the chart fails on sizes the backend would not accept. A pod whose `/tmp` grows past its `sizeLimit` is evicted, which the backend's own clone and zip limits prevent. Set `config.worker.count` in the values rather than `WORKER_COUNT` in `extraEnv`, which the chart cannot see.

The `migrate` init container shares the web pod's `/tmp`.

### Limits

The `limits` block is rendered into `config.yaml` and bounds what a single
user can make the registry do. Web and worker refuse to start when a value
does not parse, a size or count is zero or negative, `zip_max_size` exceeds
`clone_max_size` or `job_timeout` is below one minute. Sizes take binary
units (`KiB`, `MiB`, `GiB`, `TiB`, case insensitive) or plain bytes; decimal
units like `MB` are refused. Set the limits here and not through `LIMITS_*`
env vars, because the worker's termination grace period is derived from
`job_timeout`.

| Key | Default | What it bounds |
| --- | --- | --- |
| `limits.zip_max_size` | `100MiB` | Zip of one version; a larger one fails the build with a build error |
| `limits.clone_max_size` | `1GiB` | What one shallow clone or fetch writes to disk; a watchdog kills git above it. Each worker needs `worker.count` times this in temp space |
| `limits.tags_per_sync` | `50` | Release tags one sync builds, newest by version first |
| `limits.branches_per_sync` | `10` | Branches one sync builds, the default branch first |
| `limits.sync_interval` | `1m` | Minimum time between two manual syncs of one addon, across all web replicas; `0s` turns it off |
| `limits.job_timeout` | `10m` | Longest a sync job runs, git included; whole `h`, `m` and `s` only. The worker's `terminationGracePeriodSeconds` is this plus two minutes |

### Secret keys

`secrets` (or `existingSecret`) provides sensitive values as environment variables, which override the config file:

- `DATABASE_DSN`
- `STORAGE_S3_SECRET_ACCESS_KEY`
- `REDIS_URL`
- `AUTH_<PROVIDER>_CLIENT_SECRET` (per configured auth provider)
- `INTEGRATIONS_ENCRYPTION_KEY` (required once a provider sets `allow_git_integration`)

The web and worker pods and the `migrate` init container all read these from the same Secret.

### Integration encryption key

The OAuth tokens of git integrations are stored encrypted with AES-256-GCM. The key is `INTEGRATIONS_ENCRYPTION_KEY` (config key `integrations.encryption_key`): base64 of exactly 32 bytes. Generate it once and keep it with your other secrets:

```sh
openssl rand -base64 32
```

```yaml
secrets:
  INTEGRATIONS_ENCRYPTION_KEY: "..."
```

As soon as any auth provider sets `allow_git_integration: true`, the web and worker pods refuse to start without a valid key. Without such a provider the key is optional. When the key is set, the `migrate` init container encrypts tokens stored before encryption existed; running it again changes nothing. Pass the key on every `helm upgrade` (or keep it in `existingSecret`), because the rendered Secret is recreated from the values on each upgrade.

Losing or changing the key makes the stored tokens unreadable: builds of repos that clone through an integration fail until their owners connect the same account again in their profile, which overwrites the stored tokens and keeps the repos linked. Repos, addons and published versions stay as they are.

### Auth providers

Auth providers are nested config; declare them under `extraConfig.auth` and pass their secrets via `secrets`:

```yaml
extraConfig:
  auth:
    github:
      type: github
      allow_login: true
      client_id: "..."
secrets:
  AUTH_GITHUB_CLIENT_SECRET: "..."
```

### Network policies

`networkPolicy.enabled: true` renders NetworkPolicies for the release's web, worker and frontend pods. It needs a CNI that enforces them (Calico, Cilium and others; flannel alone does not). Probes from the kubelet are not affected.

- A default deny of all ingress and egress.
- Ingress to web (port `web.port`) and frontend (port `frontend.port`) from `networkPolicy.ingressFrom`, and to web from the `helm test` pod.
- Egress from web and worker to DNS, Postgres, S3 and the internet; from web also to Redis when `session.store` is `redis`. The frontend gets no egress.

| Key | Default | Description |
| --- | --- | --- |
| `networkPolicy.enabled` | `false` | Render the policies |
| `networkPolicy.ingressFrom` | namespace `ingress-nginx` | Peers of the ingress controller |
| `networkPolicy.dns.to` | `k8s-app: kube-dns` in `kube-system` | Peers of the cluster DNS |
| `networkPolicy.dns.port` | `53` | DNS port, UDP and TCP |
| `networkPolicy.postgres.to` | `[]` | Peers of Postgres; required when enabled |
| `networkPolicy.postgres.port` | `5432` | Postgres port |
| `networkPolicy.redis.to` | `[]` | Peers of Redis; required when enabled and `session.store` is `redis` |
| `networkPolicy.redis.port` | `6379` | Redis port |
| `networkPolicy.s3.to` | `[]` | Peers of S3 in the cluster or a private network; leave empty for S3 on a public address |
| `networkPolicy.s3.ports` | `[443]` | S3 ports |
| `networkPolicy.internet.enabled` | `true` | Egress from web and worker to public addresses |
| `networkPolicy.internet.ports` | `[443, 22]` | Ports of that egress: git over https and ssh, OAuth providers, public S3 |
| `networkPolicy.internet.ipBlocks` | `0.0.0.0/0` and `::/0` without private ranges | Destinations of that egress |
| `networkPolicy.extraEgress` | `[]` | Further egress rules for web and worker |

Peers are NetworkPolicyPeer lists: `podSelector`, `namespaceSelector` or `ipBlock`. An empty `to` in a NetworkPolicy allows every destination, so the chart fails instead of rendering one for Postgres or Redis:

```yaml
networkPolicy:
  enabled: true
  ingressFrom:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: traefik
  postgres:
    to:
      - namespaceSelector:
          matchLabels:
            kubernetes.io/metadata.name: databases
        podSelector:
          matchLabels:
            cnpg.io/cluster: acme-registry
  redis:
    to:
      - ipBlock:
          cidr: 10.20.0.5/32
```

The internet egress leaves out the loopback, private, link-local, CGNAT and "this network" ranges. It is the second line behind the registry's own SSRF check, which resolves every git host and refuses internal addresses before each clone and ls-remote. A git server, identity provider or S3 on a private address therefore needs its own rule in `extraEgress` or `s3.to`; the SSRF check refuses git hosts on private addresses regardless.

