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
| `web.replicaCount` | `1` | Web replicas |
| `web.autoscaling.enabled` | `false` | HPA for the web deployment |
| `worker.replicaCount` | `1` | Worker replicas |
| `worker.healthPort` | `6970` | Worker health endpoint port |
| `migration.enabled` | `true` | Run migrations in the `migrate` init container of the web pods |
| `migration.resources` | `{}` | Resources of the `migrate` init container |
| `config.base_url` | `http://localhost` | Public base URL, must be set |
| `config.instance.mode` | `public` | `public` or `private` |
| `config.storage.driver` | `s3` | Storage driver |
| `config.storage.s3.*` | | S3 endpoint, region, bucket, access_key_id, flags |
| `config.session.store` | `redis` | `redis`, or `memory` together with `dev_mode: true` |
| `config.auth.cookie_secure` | unset | Secure session cookies; unset, true when `base_url` is https |
| `extraConfig` | `{}` | Deep-merged over `config`, use for auth providers |
| `secrets` | `{}` | Secret env vars rendered into a Secret |
| `existingSecret` | `""` | Use a pre-existing Secret instead of `secrets` |
| `extraEnv` | `[]` | Extra env vars for all backend pods |
| `extraEnvFrom` | `[]` | Extra envFrom sources |
| `ingress.enabled` | `false` | Path-split ingress (API to web, rest to frontend) |

### Database migrations

Every web pod runs `migrate` (the same backend image, Secret and `config.yaml`) as an init container before the web container starts. Pods that start together, after a scale up or during a rolling update, serialize on a Postgres advisory lock: the first one migrates, the others wait and then find nothing left to do. The web container itself never migrates (`AUTO_MIGRATE=false`). This is deliberately not a Helm hook: a pre-install hook runs before the Secret and ConfigMap of the same release exist, so it cannot start on a fresh install.

During a rolling update the old pods keep serving while a new pod migrates, so a release only adds to the schema in ways the previous release can live with.

The worker does not migrate. Until a migration has created the schema it waits, checking with a backoff of up to 30 seconds, and starts taking jobs afterwards.

Set `migration.enabled: false` only if you run `migrate` yourself, against the same database and with the same config and Secret, before every upgrade.

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
