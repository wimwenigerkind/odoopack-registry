import { type FormEvent, useEffect, useMemo, useState } from "react"
import { Link, useSearchParams } from "react-router"
import { useRegisterAddon } from "@/hooks/addons/use-register-addon"
import { useMe } from "@/hooks/auth/use-me"
import { useIntegrations } from "@/hooks/integrations/use-integrations"
import { useMyRepos } from "@/hooks/repos/use-my-repos"
import type { Visibility } from "@/lib/types"

type RepoMode = "existing" | "new"

export default function AddonNewPage() {
  const { data: user, isLoading: meLoading } = useMe()
  const { data: integrations } = useIntegrations()
  const { data: myRepos } = useMyRepos()
  const register = useRegisterAddon()
  const [searchParams] = useSearchParams()

  const preselectedRepoID = searchParams.get("repo_id") ?? ""

  // Repo section
  const [repoMode, setRepoMode] = useState<RepoMode>("new")
  const [selectedRepoID, setSelectedRepoID] = useState("")
  const [gitUrl, setGitUrl] = useState("")
  const [defaultBranch, setDefaultBranch] = useState("main")
  const [integrationID, setIntegrationID] = useState("")

  // Addon section
  const [name, setName] = useState("")
  const [subpath, setSubpath] = useState("")
  const [visibility, setVisibility] = useState<Visibility>("public")

  const hasRepos = (myRepos?.length ?? 0) > 0

  useEffect(() => {
    if (!myRepos) return
    if (preselectedRepoID && myRepos.some((r) => r.id === preselectedRepoID)) {
      setRepoMode("existing")
      setSelectedRepoID(preselectedRepoID)
    } else if (hasRepos && repoMode === "new" && !preselectedRepoID) {
      // default: existing if user has any, otherwise new
      setRepoMode("existing")
      setSelectedRepoID(myRepos[0].id)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [myRepos])

  const selectedRepo = useMemo(
    () => myRepos?.find((r) => r.id === selectedRepoID),
    [myRepos, selectedRepoID],
  )

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()

    const repoFields =
      repoMode === "existing" && selectedRepo
        ? {
            git_url: selectedRepo.git_url,
            default_branch: selectedRepo.default_branch,
            integration_id: selectedRepo.integration_id ?? undefined,
          }
        : {
            git_url: gitUrl,
            default_branch: defaultBranch,
            integration_id: integrationID || undefined,
          }

    register.mutate({
      name,
      ...repoFields,
      subpath: subpath || undefined,
      visibility,
    })
  }

  if (meLoading) return <p>Loading…</p>
  if (!user) return <p>You must be logged in to register an addon.</p>

  if (register.isSuccess) {
    const addon = register.data
    return (
      <>
        <h2>Addon registered</h2>
        <p>
          <strong>{addon.name}</strong> has been registered.
        </p>
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
        <fieldset>
          <legend>Repo</legend>

          {hasRepos && (
            <p>
              <label>
                <input
                  type="radio"
                  name="repo-mode"
                  value="existing"
                  checked={repoMode === "existing"}
                  onChange={() => setRepoMode("existing")}
                />{" "}
                Use existing repo
              </label>
              {"  "}
              <label>
                <input
                  type="radio"
                  name="repo-mode"
                  value="new"
                  checked={repoMode === "new"}
                  onChange={() => setRepoMode("new")}
                />{" "}
                Register new repo
              </label>
            </p>
          )}

          {repoMode === "existing" && hasRepos ? (
            <p>
              <label>
                Repo
                <br />
                <select
                  required
                  value={selectedRepoID}
                  onChange={(e) => setSelectedRepoID(e.target.value)}
                >
                  <option value="">— Select a repo —</option>
                  {myRepos!.map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.git_url} ({r.default_branch})
                    </option>
                  ))}
                </select>
              </label>
              {selectedRepo && (
                <>
                  <br />
                  <small>
                    Default branch: <code>{selectedRepo.default_branch}</code>
                    {" · "}
                    Integration:{" "}
                    {selectedRepo.integration
                      ? `${selectedRepo.integration.provider}${selectedRepo.integration.account_name ? ` (${selectedRepo.integration.account_name})` : ""}`
                      : "none"}
                  </small>
                </>
              )}
            </p>
          ) : (
            <>
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
            </>
          )}
        </fieldset>

        <fieldset>
          <legend>Addon</legend>

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
              Subpath
              <br />
              <input
                value={subpath}
                onChange={(e) => setSubpath(e.target.value)}
                placeholder="(leave empty if manifest is at repo root)"
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
        </fieldset>

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
