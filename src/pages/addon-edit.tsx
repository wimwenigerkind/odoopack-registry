import { type FormEvent, useEffect, useState } from "react"
import { Link, useNavigate, useParams } from "react-router"
import { useAddon } from "@/hooks/addons/use-addon"
import { useUpdateAddon } from "@/hooks/addons/use-update-addon"
import type { Visibility } from "@/lib/types"

export default function AddonEditPage() {
  const { id = "" } = useParams()
  const { data: addon, isLoading, isError } = useAddon(id)
  const update = useUpdateAddon(id)
  const navigate = useNavigate()

  const [subpath, setSubpath] = useState("")
  const [visibility, setVisibility] = useState<Visibility>("public")

  useEffect(() => {
    if (!addon) return
    setSubpath(addon.subpath ?? "")
    setVisibility(addon.visibility)
  }, [addon])

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    update.mutate(
      { subpath, visibility },
      { onSuccess: () => navigate(`/addons/${id}`) },
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

      <p>
        <small>
          Repo-level settings (git URL, default branch, integration) live on the{" "}
          <Link to={`/repos/${addon.repo_id}`}>repo</Link>.
        </small>
      </p>

      <form onSubmit={handleSubmit}>
        <p>
          <label>
            Subpath
            <br />
            <input
              value={subpath}
              onChange={(e) => setSubpath(e.target.value)}
              placeholder="(empty = repo root)"
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

        {update.isError && <p>Update failed: {update.error.message}</p>}

        <button type="submit" disabled={update.isPending}>
          {update.isPending ? "Saving…" : "Save"}
        </button>
      </form>
    </>
  )
}
