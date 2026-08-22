import { ArrowLeft, CheckCircle2 } from "lucide-react"
import { useState } from "react"
import type { FormEvent } from "react"
import { Link, useSearchParams } from "react-router"
import {
  Button,
  buttonVariants,
  Card,
  Field,
  Input,
  Select,
  Spinner,
} from "@/components/ui"
import { useRegisterAddon } from "@/hooks/addons/use-register-addon"
import { useMe } from "@/hooks/auth/use-me"
import { useIntegrations } from "@/hooks/integrations/use-integrations"
import { useMyRepos } from "@/hooks/repos/use-my-repos"
import type { OAuthIntegration, Repo, Visibility } from "@/lib/types"

type RepoMode = "existing" | "new"

export default function AddonNewPage() {
  const { data: user, isLoading: meLoading } = useMe()
  const { data: myRepos, isLoading: reposLoading } = useMyRepos()
  const { data: integrations } = useIntegrations()
  const [searchParams] = useSearchParams()

  if (meLoading || reposLoading)
    return (
      <div className="flex justify-center py-16">
        <Spinner className="size-6" />
      </div>
    )
  if (!user)
    return <p>You must be logged in to register an addon.</p>

  return (
    <NewAddonForm
      repos={myRepos ?? []}
      integrations={integrations ?? []}
      preselectedRepoID={searchParams.get("repo_id") ?? ""}
    />
  )
}

function NewAddonForm({
  repos,
  integrations,
  preselectedRepoID,
}: {
  repos: Repo[]
  integrations: OAuthIntegration[]
  preselectedRepoID: string
}) {
  const register = useRegisterAddon()
  const hasRepos = repos.length > 0
  const preselected = repos.some((r) => r.id === preselectedRepoID)
    ? preselectedRepoID
    : ""

  const [repoMode, setRepoMode] = useState<RepoMode>(
    hasRepos ? "existing" : "new",
  )
  const [selectedRepoID, setSelectedRepoID] = useState(
    preselected || (hasRepos ? repos[0].id : ""),
  )
  const [gitUrl, setGitUrl] = useState("")
  const [defaultBranch, setDefaultBranch] = useState("main")
  const [integrationID, setIntegrationID] = useState("")
  const [name, setName] = useState("")
  const [subpath, setSubpath] = useState("")
  const [visibility, setVisibility] = useState<Visibility>("public")

  const selectedRepo = repos.find((r) => r.id === selectedRepoID)

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

  if (register.isSuccess) {
    const addon = register.data
    return (
      <div className="mx-auto max-w-xl">
        <Card className="flex flex-col items-center gap-3 p-8 text-center">
          <CheckCircle2 className="size-10 text-success" />
          <h1 className="text-xl font-semibold">Addon registered</h1>
          <p className="text-sm text-muted">
            <strong className="text-fg">{addon.name}</strong> has been
            registered.
          </p>
          <div className="mt-2 flex gap-2">
            <Link to={`/addons/${addon.id}`} className={buttonVariants()}>
              Go to addon
            </Link>
            <Link
              to="/"
              className={buttonVariants({ variant: "secondary" })}
            >
              Back to overview
            </Link>
          </div>
        </Card>
      </div>
    )
  }

  return (
    <div className="mx-auto flex max-w-xl flex-col gap-6">
      <Link
        to="/"
        className="inline-flex w-fit items-center gap-1 text-sm text-muted transition-colors hover:text-fg"
      >
        <ArrowLeft className="size-4" />
        Back
      </Link>
      <h1 className="text-2xl font-semibold">Register addon</h1>

      <form onSubmit={handleSubmit} className="flex flex-col gap-6">
        <Card className="flex flex-col gap-4 p-5">
          <h2 className="font-medium">Repository</h2>

          {hasRepos && (
            <div className="flex gap-4 text-sm">
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="repo-mode"
                  className="accent-accent"
                  checked={repoMode === "existing"}
                  onChange={() => setRepoMode("existing")}
                />
                Use existing repo
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="repo-mode"
                  className="accent-accent"
                  checked={repoMode === "new"}
                  onChange={() => setRepoMode("new")}
                />
                Register new repo
              </label>
            </div>
          )}

          {repoMode === "existing" && hasRepos ? (
            <Field label="Repo" htmlFor="repo">
              <Select
                id="repo"
                required
                value={selectedRepoID}
                onChange={(e) => setSelectedRepoID(e.target.value)}
              >
                <option value="">Select a repo</option>
                {repos.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.git_url} ({r.default_branch})
                  </option>
                ))}
              </Select>
            </Field>
          ) : (
            <>
              <Field label="Git URL" htmlFor="git-url">
                <Input
                  id="git-url"
                  required
                  value={gitUrl}
                  onChange={(e) => setGitUrl(e.target.value)}
                  placeholder="https://git.example.com/org/repo.git"
                />
              </Field>
              <Field label="Default branch" htmlFor="default-branch">
                <Input
                  id="default-branch"
                  value={defaultBranch}
                  onChange={(e) => setDefaultBranch(e.target.value)}
                />
              </Field>
              <Field label="Integration" htmlFor="integration">
                <Select
                  id="integration"
                  value={integrationID}
                  onChange={(e) => setIntegrationID(e.target.value)}
                >
                  <option value="">None (anonymous clone)</option>
                  {integrations.map((i) => (
                    <option key={i.id} value={i.id}>
                      {i.provider}
                      {i.account_name ? ` (${i.account_name})` : ""}
                    </option>
                  ))}
                </Select>
              </Field>
            </>
          )}
        </Card>

        <Card className="flex flex-col gap-4 p-5">
          <h2 className="font-medium">Addon</h2>
          <Field label="Name" htmlFor="name">
            <Input
              id="name"
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="vendor/name"
            />
          </Field>
          <Field
            label="Subpath"
            htmlFor="subpath"
            hint="Leave empty if the manifest is at the repo root."
          >
            <Input
              id="subpath"
              value={subpath}
              onChange={(e) => setSubpath(e.target.value)}
              placeholder="addons/my_module"
            />
          </Field>
          <Field label="Visibility" htmlFor="visibility">
            <Select
              id="visibility"
              value={visibility}
              onChange={(e) => setVisibility(e.target.value as Visibility)}
            >
              <option value="public">Public</option>
              <option value="private">Private</option>
            </Select>
          </Field>
        </Card>

        {register.isError && (
          <p className="text-sm text-danger">
            Registration failed: {register.error.message}
          </p>
        )}

        <div className="flex justify-end">
          <Button type="submit" loading={register.isPending}>
            Register addon
          </Button>
        </div>
      </form>
    </div>
  )
}
