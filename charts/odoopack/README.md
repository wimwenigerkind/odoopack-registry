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
  --set secrets.STORAGE_S3_SECRET_ACCESS_KEY=SECRET
```

## Prerequisites

- An external PostgreSQL database (pass its DSN via `secrets.DATABASE_DSN` or `existingSecret`).
- S3-compatible object storage, shared by the web and worker pods.
- Optional external Redis when `config.session.store` is `redis`.

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
| `migration.enabled` | `true` | Run migrations as a pre-upgrade Job; web skips auto-migrate |
| `config.base_url` | `http://localhost` | Public base URL, must be set |
| `config.instance.mode` | `public` | `public` or `private` |
| `config.storage.driver` | `s3` | Storage driver |
| `config.storage.s3.*` | | S3 endpoint, region, bucket, access_key_id, flags |
| `config.session.store` | `memory` | `memory` or `redis` |
| `config.auth.cookie_secure` | `true` | Secure session cookies |
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
- `REDIS_URL` (when using Redis)
- `AUTH_<PROVIDER>_CLIENT_SECRET` (per configured auth provider)

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
