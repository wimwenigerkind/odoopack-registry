# odoopack Helm chart

Deploys the odoopack registry: the web API, the background worker, the frontend, and a one-shot database migration Job.

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
| `worker.terminationGracePeriodSeconds` | `720` | Time a stopping worker gets to finish its running job; keep it above the 10 minute job timeout |
| `migration.enabled` | `true` | Run migrations as a pre-upgrade Job; web skips auto-migrate |
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

### Secret keys

`secrets` (or `existingSecret`) provides sensitive values as environment variables, which override the config file:

- `DATABASE_DSN`
- `STORAGE_S3_SECRET_ACCESS_KEY`
- `REDIS_URL`
- `AUTH_<PROVIDER>_CLIENT_SECRET` (per configured auth provider)
- `INTEGRATIONS_ENCRYPTION_KEY` (required once a provider sets `allow_git_integration`)

The web, worker and migration pods all read these from the same Secret.

### Integration encryption key

The OAuth tokens of git integrations are stored encrypted with AES-256-GCM. The key is `INTEGRATIONS_ENCRYPTION_KEY` (config key `integrations.encryption_key`): base64 of exactly 32 bytes. Generate it once and keep it with your other secrets:

```sh
openssl rand -base64 32
```

```yaml
secrets:
  INTEGRATIONS_ENCRYPTION_KEY: "..."
```

As soon as any auth provider sets `allow_git_integration: true`, the web and worker pods refuse to start without a valid key. Without such a provider the key is optional. When the key is set, the migration Job encrypts tokens stored before encryption existed; running it again changes nothing. Pass the key on every `helm upgrade` (or keep it in `existingSecret`), because the rendered Secret is recreated from the values on each upgrade.

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
