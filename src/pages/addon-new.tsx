import { type FormEvent, useState } from "react"
import { Link } from "react-router"
import { useRegisterAddon } from "@/hooks/addons/use-register-addon"
import { useMe } from "@/hooks/auth/use-me"
import type { Visibility } from "@/lib/types"

export default function AddonNewPage() {
  const { data: user, isLoading: meLoading } = useMe()
  const register = useRegisterAddon()

  const [name, setName] = useState("")
  const [gitUrl, setGitUrl] = useState("")
  const [defaultBranch, setDefaultBranch] = useState("main")
  const [visibility, setVisibility] = useState<Visibility>("public")

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    register.mutate({
      name,
      git_url: gitUrl,
      default_branch: defaultBranch,
      visibility,
    })
  }

  if (meLoading) return <p>Loading…</p>
  if (!user) return <p>You must be logged in to register an addon.</p>

  if (register.isSuccess) {
    const { addon, webhook_secret } = register.data
    return (
      <>
        <h2>Addon registered</h2>
        <p>
          <strong>{addon.name}</strong> has been registered.
        </p>

        <section>
          <h3>Webhook secret</h3>
          <p>
            Store this somewhere safe. It cannot be retrieved again. You will
            need it later when configuring provider-specific webhooks.
          </p>
          <p>
            <code>{webhook_secret}</code>{" "}
            <button
              type="button"
              onClick={() =>
                navigator.clipboard?.writeText(webhook_secret)
              }
            >
              Copy
            </button>
          </p>
        </section>

        <p>
          <Link to={`/addons/${addon.id}`}>Go to addon</Link>
          {" | "}
          <Link to="/">Back to overview</Link>
        </p>
      </>
    )
  }

  return (
    <>
      <p>
        <Link to="/">Back</Link>
      </p>
      <h2>Register addon</h2>

      <form onSubmit={handleSubmit}>
        <p>
          <label>
            Name
            <br />
            <input
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="vendor/name"
            />
          </label>
        </p>

        <p>
          <label>
            Git URL
            <br />
            <input
              required
              value={gitUrl}
              onChange={(e) => setGitUrl(e.target.value)}
              placeholder="https://bitbucket.org/org/repo.git"
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

        {register.isError && (
          <p>Registration failed: {register.error.message}</p>
        )}

        <button type="submit" disabled={register.isPending}>
          {register.isPending ? "Registering…" : "Register"}
        </button>
      </form>
    </>
  )
}
