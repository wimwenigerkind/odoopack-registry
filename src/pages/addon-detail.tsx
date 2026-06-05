import { Link, useParams } from "react-router"
import { useAddon } from "@/hooks/addons/use-addon"
import { useMe } from "@/hooks/auth/use-me"
import { useSyncAddon } from "@/hooks/addons/use-sync-addon"
import { Avatar } from "@/components/avatar"
import type { VersionStatus } from "@/lib/types"

export default function AddonDetailPage() {
  const { id = "" } = useParams()
  const { data: addon, isLoading, isError } = useAddon(id)
  const { data: user } = useMe()
  const sync = useSyncAddon(id)

  if (isLoading) return <p>Loading…</p>
  if (isError) return <p>Could not load addon</p>
  if (!addon) return <p>Addon not found</p>

  const isOwner = user?.id === addon.owner_id
  const versions = addon.versions ?? []

  return (
    <>
      <p>
        <Link to="/">Back</Link>
      </p>

      <h2>{addon.name}</h2>

      <dl>
        <dt>Owner</dt>
        <dd style={{ display: "flex", alignItems: "center", gap: 6 }}>
          {addon.owner?.email && <Avatar email={addon.owner.email} size={24} />}
          {addon.owner?.username ?? "-"}
        </dd>
        <dt>Visibility</dt>
        <dd>{addon.visibility}</dd>
        <dt>Git URL</dt>
        <dd>
          <code>{addon.git_url}</code>
        </dd>
        <dt>Default branch</dt>
        <dd>{addon.default_branch}</dd>
        <dt>Provider</dt>
        <dd>{addon.git_provider}</dd>
      </dl>

      {isOwner && (
        <section>
          <button onClick={() => sync.mutate()} disabled={sync.isPending}>
            {sync.isPending ? "Syncing…" : "Sync now"}
          </button>
          {sync.isError && <p>Sync failed</p>}
          {sync.isSuccess && <p>Sync queued</p>}
        </section>
      )}

      <h3>Versions ({versions.length})</h3>
      {versions.length === 0 ? (
        <p>No versions yet. Sync the addon to discover tags and branches.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Version</th>
              <th>Ref</th>
              <th>Status</th>
              <th>Size</th>
              <th>Built</th>
              <th>Download</th>
            </tr>
          </thead>
          <tbody>
            {versions.map((v) => (
              <tr key={v.id}>
                <td>{v.version}</td>
                <td>
                  <small>
                    {v.ref_type}: <code>{v.ref_value.slice(0, 8)}</code>
                  </small>
                </td>
                <td>
                  <StatusBadge status={v.status} />
                </td>
                <td>{formatSize(v.size_bytes)}</td>
                <td>
                  {v.built_at ? new Date(v.built_at).toLocaleString() : "-"}
                </td>
                <td>
                  {v.status === "ready" ? (
                    <a
                      href={`/api/v1/addons/${addon.id}/versions/${encodeURIComponent(v.version)}/download`}
                    >
                      Download
                    </a>
                  ) : (
                    "-"
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  )
}

function StatusBadge({ status }: { status: VersionStatus }) {
  return <span data-status={status}>{status}</span>
}

function formatSize(bytes?: number): string {
  if (!bytes) return "-"
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}
