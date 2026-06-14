import { type FormEvent, useEffect, useState } from "react"
import { Link, useNavigate, useParams } from "react-router"
import { useAddon } from "@/hooks/addons/use-addon"
import { useUpdateAddon } from "@/hooks/addons/use-update-addon"
import { useIntegrations } from "@/hooks/integrations/use-integrations"
import type { Visibility } from "@/lib/types"

export default function AddonEditPage() {
  const { id = "" } = useParams()
  const { data: addon, isLoading, isError } = useAddon(id)
  const { data: integrations } = useIntegrations()
  const update = useUpdateAddon(id)
  const navigate = useNavigate()

  const [gitUrl, setGitUrl] = useState("")
  const [defaultBranch, setDefaultBranch] = useState("")
  const [visibility, setVisibility] = useState<Visibility>("public")
  const [integrationID, setIntegrationID] = useState("")

  useEffect(() => {
    if (!addon) return
    setGitUrl(addon.git_url)
    setDefaultBranch(addon.default_branch)
    setVisibility(addon.visibility)
    setIntegrationID(addon.integration_id ?? "")
  }, [addon])

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    update.mutate(
      {
        git_url: gitUrl,
        default_branch: defaultBranch,
        visibility,
        integration_id: integrationID || null,
      },
      {
        onSuccess: () => navigate(`/addons/${id}`),
      },
    )
  }

  if (isLoading) return <p>Loading…</p>
  if (isError || !addon) return <p>Could not load addon.</p>

  return (
    <>
      <p>
        <Link to={`/addons/${id}`}>Back</Link>
      </p>
      <h2>Edit {addon.name}</h2>

      <form onSubmit={handleSubmit}>
        <p>
          <label>
            Git URL
            <br />
            <input
              required
              value={gitUrl}
              onChange={(e) => setGitUrl(e.target.value)}
            />
          </label>
        </p>

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
            Visibility
            <br />
            <select
              value={visibility}
              onChange={(e) => setVisibility(e.target.value as Visibility)}
            >
              <option value="public">Public</option>
              <option value="private">Private</option>
            </select>
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

        <button type="submit" disabled={update.isPending}>
          {update.isPending ? "Saving…" : "Save"}
        </button>
      </form>
    </>
  )
}
