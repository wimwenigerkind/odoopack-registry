import { useMemo, useState } from "react"
import { Link, useNavigate, useParams } from "react-router"
import { useAddon } from "@/hooks/addons/use-addon"
import { useAddonReadme } from "@/hooks/addons/use-addon-readme"
import { useMe } from "@/hooks/auth/use-me"
import { useSyncAddon } from "@/hooks/addons/use-sync-addon"
import { useDeleteVersion } from "@/hooks/addons/use-delete-version"
import { useDeleteAddon } from "@/hooks/addons/use-delete-addon"
import { Avatar } from "@/components/avatar"
import type { AddonVersion, VersionStatus } from "@/lib/types"

export default function AddonDetailPage() {
  const { id = "" } = useParams()
  const { data: addon, isLoading, isError } = useAddon(id)
  const { data: user } = useMe()
  const sync = useSyncAddon(id)
  const deleteVersion = useDeleteVersion(id)
  const deleteAddon = useDeleteAddon()
  const navigate = useNavigate()

  if (isLoading) return <p>Loading…</p>
  if (isError) return <p>Could not load addon</p>
  if (!addon) return <p>Addon not found</p>

  const isOwner = user?.id === addon.repo?.owner_id
  const versions = addon.versions ?? []

  return (
    <>
      <p>
        <Link to="/">Back</Link>
      </p>

      <h2>{addon.name}</h2>

      <InstallSnippet name={addon.name} />

      <ReadmeSection addonId={addon.id} versions={versions} />

      <dl>
        <dt>Owner</dt>
        <dd style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <Avatar hash={addon.repo?.owner?.gravatar_hash} size={24} />
          {addon.repo?.owner?.username ?? "-"}
        </dd>
        <dt>Visibility</dt>
        <dd>{addon.visibility}</dd>
        <dt>Repo</dt>
        <dd>
          {addon.repo ? (
            <Link to={`/repos/${addon.repo.id}`}>
              <code>{addon.repo.git_url}</code>
            </Link>
          ) : (
            "-"
          )}
        </dd>
        <dt>Subpath</dt>
        <dd>{addon.subpath ? <code>{addon.subpath}</code> : "(repo root)"}</dd>
        <dt>Default branch</dt>
        <dd>{addon.repo?.default_branch ?? "-"}</dd>
        <dt>Integration</dt>
        <dd>
          {addon.repo?.integration
            ? `${addon.repo.integration.provider}${addon.repo.integration.account_name ? ` (${addon.repo.integration.account_name})` : ""}`
            : "none (anonymous clone)"}
        </dd>
      </dl>

      {isOwner && (
        <section>
          <button onClick={() => sync.mutate()} disabled={sync.isPending}>
            {sync.isPending ? "Syncing…" : "Sync now"}
          </button>
          {" "}
          <Link to={`/addons/${addon.id}/edit`}>Edit</Link>
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
              {isOwner && <th></th>}
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
                {isOwner && (
                  <td>
                    <button
                      onClick={() => {
                        if (confirm(`Delete version "${v.version}"?`)) {
                          deleteVersion.mutate(v.version)
                        }
                      }}
                      disabled={deleteVersion.isPending}
                    >
                      Delete
                    </button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {isOwner && (
        <section>
          <h3>Danger zone</h3>
          <p>
            <small>
              Removes the addon, all its versions, and built zipballs. The
              underlying repo stays.
            </small>
          </p>
          <button
            disabled={deleteAddon.isPending}
            onClick={() => {
              if (confirm(`Delete addon "${addon.name}"?`)) {
                deleteAddon.mutate(addon.id, {
                  onSuccess: () => navigate("/"),
                })
              }
            }}
          >
            {deleteAddon.isPending ? "Deleting…" : "Delete addon"}
          </button>
          {deleteAddon.isError && (
            <p>Delete failed: {deleteAddon.error.message}</p>
          )}
        </section>
      )}
    </>
  )
}

function ReadmeSection({
  addonId,
  versions,
}: {
  addonId: string
  versions: AddonVersion[]
}) {
  const readable = useMemo(() => latestFirst(versions), [versions])
  const [selected, setSelected] = useState<string>("")
  const version = selected || readable[0]?.version || ""
  const { data, isLoading, isError } = useAddonReadme(addonId, version)

  if (readable.length === 0) return null

  return (
    <section>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <h3 style={{ margin: 0 }}>README</h3>
        <select value={version} onChange={(e) => setSelected(e.target.value)}>
          {readable.map((v) => (
            <option key={v.id} value={v.version}>
              {v.version}
            </option>
          ))}
        </select>
      </div>
      {isLoading ? (
        <p>Loading README…</p>
      ) : isError ? (
        <p>Could not load README</p>
      ) : !data ? (
        <p>No README for this version.</p>
      ) : (
        <div
          className="readme"
          dangerouslySetInnerHTML={{ __html: data.html }}
        />
      )}
    </section>
  )
}

function latestFirst(versions: AddonVersion[]): AddonVersion[] {
  return versions
    .filter((v) => v.status === "ready")
    .sort((a, b) => {
      const ta = a.built_at ? Date.parse(a.built_at) : 0
      const tb = b.built_at ? Date.parse(b.built_at) : 0
      return tb - ta
    })
}

function InstallSnippet({ name }: { name: string }) {
  const cmd = `odoopack add ${name}`
  return (
    <p>
      <code>{cmd}</code>{" "}
      <button
        type="button"
        onClick={() => navigator.clipboard?.writeText(cmd)}
      >
        Copy
      </button>
    </p>
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
