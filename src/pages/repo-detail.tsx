import { type FormEvent, useEffect, useState } from "react"
import { Link, useNavigate, useParams } from "react-router"
import { useRepo } from "@/hooks/repos/use-repo"
import { useUpdateRepo } from "@/hooks/repos/use-update-repo"
import { useDeleteRepo } from "@/hooks/repos/use-delete-repo"
import { useMe } from "@/hooks/auth/use-me"
import { useIntegrations } from "@/hooks/integrations/use-integrations"
import { Avatar } from "@/components/avatar"

export default function RepoDetailPage() {
  const { id = "" } = useParams()
  const { data: repo, isLoading, isError } = useRepo(id)
  const { data: user } = useMe()
  const navigate = useNavigate()

  if (isLoading) return <p>Loading…</p>
  if (isError) return <p>Could not load repo.</p>
  if (!repo) return <p>Repo not found.</p>

  const isOwner = user?.id === repo.owner_id
  const addons = repo.addons ?? []

  return (
    <>
      <p>
        <Link to="/">Back</Link>
      </p>

      <h2>Repo</h2>

      <dl>
        <dt>Git URL</dt>
        <dd>
          <code>{repo.git_url}</code>
        </dd>
        <dt>Default branch</dt>
        <dd>{repo.default_branch}</dd>
        <dt>Owner</dt>
        <dd style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <Avatar hash={repo.owner?.gravatar_hash} size={24} />
          {repo.owner?.username ?? "-"}
        </dd>
        {isOwner && (
          <>
            <dt>Integration</dt>
            <dd>
              {repo.integration
                ? `${repo.integration.provider}${repo.integration.account_name ? ` (${repo.integration.account_name})` : ""}`
                : "none (anonymous clone)"}
            </dd>
          </>
        )}
      </dl>

      {isOwner && <RepoSettings repo={repo} />}

      <h3>Addons in this repo ({addons.length})</h3>
      {isOwner && (
        <p>
          <Link to={`/addons/new?repo_id=${repo.id}`}>
            + Add addon to this repo
          </Link>
        </p>
      )}
      {addons.length === 0 ? (
        <p>No addons registered under this repo yet.</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Subpath</th>
              <th>Visibility</th>
              <th>Versions</th>
            </tr>
          </thead>
          <tbody>
            {addons.map((a) => (
              <tr key={a.id}>
                <td>
                  <Link to={`/addons/${a.id}`}>{a.name}</Link>
                </td>
                <td>
                  {a.subpath ? <code>{a.subpath}</code> : "(repo root)"}
                </td>
                <td>{a.visibility}</td>
                <td>{a.versions?.length ?? 0}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {isOwner && (
        <DangerZone
          repoId={repo.id}
          addonCount={addons.length}
          onDeleted={() => navigate("/profile")}
        />
      )}
    </>
  )
}

function RepoSettings({
  repo,
}: {
  repo: import("@/lib/types").Repo
}) {
  const update = useUpdateRepo(repo.id)
  const { data: integrations } = useIntegrations()
  const [defaultBranch, setDefaultBranch] = useState(repo.default_branch)
  const [integrationID, setIntegrationID] = useState(repo.integration_id ?? "")

  useEffect(() => {
    setDefaultBranch(repo.default_branch)
    setIntegrationID(repo.integration_id ?? "")
  }, [repo.id, repo.default_branch, repo.integration_id])

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    update.mutate({
      default_branch: defaultBranch,
      integration_id: integrationID || null,
    })
  }

  return (
    <section>
      <h3>Settings</h3>
      <form onSubmit={handleSubmit}>
        <p>
          <label>
            Default branch
            <br />
            <input
              value={defaultBranch}
              onChange={(e) => setDefaultBranch(e.target.value)}
            />
          </label>
        </p>

        <p>
          <label>
            Integration
            <br />
            <select
              value={integrationID}
              onChange={(e) => setIntegrationID(e.target.value)}
            >
              <option value="">None (anonymous clone)</option>
              {(integrations ?? []).map((i) => (
                <option key={i.id} value={i.id}>
                  {i.provider}
                  {i.account_name ? ` (${i.account_name})` : ""}
                </option>
              ))}
            </select>
          </label>
        </p>

        {update.isError && <p>Update failed: {update.error.message}</p>}
        {update.isSuccess && <p>Saved.</p>}

        <button type="submit" disabled={update.isPending}>
          {update.isPending ? "Saving…" : "Save"}
        </button>
      </form>
    </section>
  )
}

function DangerZone({
  repoId,
  addonCount,
  onDeleted,
}: {
  repoId: string
  addonCount: number
  onDeleted: () => void
}) {
  const del = useDeleteRepo()
  const blocked = addonCount > 0

  return (
    <section>
      <h3>Danger zone</h3>
      {blocked && (
        <p>
          <small>
            Delete the {addonCount} addon{addonCount === 1 ? "" : "s"} first to
            remove this repo.
          </small>
        </p>
      )}
      <button
        disabled={blocked || del.isPending}
        onClick={() => {
          if (confirm("Delete this repo?")) {
            del.mutate(repoId, { onSuccess: onDeleted })
          }
        }}
      >
        {del.isPending ? "Deleting…" : "Delete repo"}
      </button>
      {del.isError && <p>Delete failed: {del.error.message}</p>}
    </section>
  )
}
