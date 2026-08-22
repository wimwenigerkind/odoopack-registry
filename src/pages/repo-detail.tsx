import { ArrowLeft, Plus, Trash2 } from "lucide-react"
import { useState } from "react"
import type { FormEvent, ReactNode } from "react"
import { Link, useNavigate, useParams } from "react-router"
import { Avatar } from "@/components/avatar"
import {
  Badge,
  Button,
  buttonVariants,
  Card,
  ConfirmDialog,
  EmptyState,
  Field,
  Input,
  Select,
  Spinner,
  Table,
  TBody,
  TD,
  TH,
  THead,
  TR,
} from "@/components/ui"
import { useMe } from "@/hooks/auth/use-me"
import { useIntegrations } from "@/hooks/integrations/use-integrations"
import { useDeleteRepo } from "@/hooks/repos/use-delete-repo"
import { useRepo } from "@/hooks/repos/use-repo"
import { useUpdateRepo } from "@/hooks/repos/use-update-repo"
import type { Repo } from "@/lib/types"

export default function RepoDetailPage() {
  const { id = "" } = useParams()
  const { data: repo, isLoading, isError } = useRepo(id)
  const { data: user } = useMe()
  const navigate = useNavigate()

  if (isLoading)
    return (
      <div className="flex justify-center py-16">
        <Spinner className="size-6" />
      </div>
    )
  if (isError) return <p className="text-danger">Could not load repo.</p>
  if (!repo) return <p>Repo not found.</p>

  const isOwner = user?.id === repo.owner_id
  const addons = repo.addons ?? []

  return (
    <div className="flex flex-col gap-6">
      <Link
        to="/"
        className="inline-flex w-fit items-center gap-1 text-sm text-muted transition-colors hover:text-fg"
      >
        <ArrowLeft className="size-4" />
        Back
      </Link>

      <div>
        <h1 className="text-2xl font-semibold">Repository</h1>
        <code className="mt-1 block break-all text-sm text-muted">
          {repo.git_url}
        </code>
      </div>

      <Card className="p-5">
        <dl className="grid grid-cols-1 gap-x-8 gap-y-4 text-sm sm:grid-cols-2">
          <Detail label="Default branch">
            <code>{repo.default_branch}</code>
          </Detail>
          <Detail label="Owner">
            <span className="flex items-center gap-2">
              <Avatar hash={repo.owner?.gravatar_hash} size={20} />
              {repo.owner?.username ?? "-"}
            </span>
          </Detail>
          {isOwner && (
            <Detail label="Integration">
              {repo.integration ? (
                <span>
                  {repo.integration.provider}
                  {repo.integration.account_name
                    ? ` (${repo.integration.account_name})`
                    : ""}
                </span>
              ) : (
                <span className="text-muted">none (anonymous clone)</span>
              )}
            </Detail>
          )}
        </dl>
      </Card>

      {isOwner && <RepoSettings key={repo.id} repo={repo} />}

      <section className="flex flex-col gap-3">
        <div className="flex items-center justify-between gap-4">
          <h2 className="text-lg font-semibold">
            Addons{" "}
            <span className="font-normal text-muted">({addons.length})</span>
          </h2>
          {isOwner && (
            <Link
              to={`/addons/new?repo_id=${repo.id}`}
              className={buttonVariants({ variant: "secondary", size: "sm" })}
            >
              <Plus className="size-4" />
              Add addon
            </Link>
          )}
        </div>
        {addons.length === 0 ? (
          <EmptyState
            title="No addons yet"
            description="No addons are registered under this repo."
          />
        ) : (
          <Table>
            <THead>
              <TR>
                <TH>Name</TH>
                <TH>Subpath</TH>
                <TH>Visibility</TH>
                <TH>Versions</TH>
              </TR>
            </THead>
            <TBody>
              {addons.map((a) => (
                <TR key={a.id}>
                  <TD className="font-medium">
                    <Link to={`/addons/${a.id}`} className="hover:text-accent">
                      {a.name}
                    </Link>
                  </TD>
                  <TD className="text-muted">
                    {a.subpath ? (
                      <code>{a.subpath}</code>
                    ) : (
                      "(repo root)"
                    )}
                  </TD>
                  <TD>
                    <Badge
                      variant={a.visibility === "public" ? "neutral" : "warning"}
                    >
                      {a.visibility}
                    </Badge>
                  </TD>
                  <TD className="text-muted">{a.versions?.length ?? 0}</TD>
                </TR>
              ))}
            </TBody>
          </Table>
        )}
      </section>

      {isOwner && (
        <DangerZone
          repoId={repo.id}
          addonCount={addons.length}
          onDeleted={() => navigate("/profile")}
        />
      )}
    </div>
  )
}

function Detail({
  label,
  children,
}: {
  label: string
  children: ReactNode
}) {
  return (
    <div className="flex flex-col gap-1">
      <dt className="text-xs font-medium uppercase tracking-wide text-muted">
        {label}
      </dt>
      <dd>{children}</dd>
    </div>
  )
}

function RepoSettings({ repo }: { repo: Repo }) {
  const update = useUpdateRepo(repo.id)
  const { data: integrations } = useIntegrations()
  const [defaultBranch, setDefaultBranch] = useState(repo.default_branch)
  const [integrationID, setIntegrationID] = useState(repo.integration_id ?? "")

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    update.mutate({
      default_branch: defaultBranch,
      integration_id: integrationID || null,
    })
  }

  return (
    <Card className="p-5">
      <h2 className="mb-4 font-medium">Settings</h2>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
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
            {(integrations ?? []).map((i) => (
              <option key={i.id} value={i.id}>
                {i.provider}
                {i.account_name ? ` (${i.account_name})` : ""}
              </option>
            ))}
          </Select>
        </Field>

        <div className="flex justify-end">
          <Button type="submit" loading={update.isPending}>
            Save
          </Button>
        </div>
      </form>
    </Card>
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
    <Card className="border-danger/30 p-5">
      <h2 className="text-base font-semibold text-danger">Danger zone</h2>
      <p className="mt-1 text-sm text-muted">
        {blocked
          ? `Delete the ${addonCount} addon${addonCount === 1 ? "" : "s"} first to remove this repo.`
          : "Permanently removes this repo. This cannot be undone."}
      </p>
      <div className="mt-4">
        <ConfirmDialog
          trigger={
            <Button variant="danger" disabled={blocked} loading={del.isPending}>
              <Trash2 className="size-4" />
              Delete repo
            </Button>
          }
          title="Delete this repo?"
          description="This cannot be undone."
          confirmLabel="Delete repo"
          destructive
          loading={del.isPending}
          onConfirm={() => del.mutate(repoId, { onSuccess: onDeleted })}
        />
      </div>
      {del.isError && (
        <p className="mt-2 text-sm text-danger">
          Delete failed: {del.error.message}
        </p>
      )}
    </Card>
  )
}
