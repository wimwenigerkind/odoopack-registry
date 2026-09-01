import {
  ArrowLeft,
  Boxes,
  Download,
  FileText,
  Package,
  Pencil,
  RefreshCw,
  Trash2,
} from "lucide-react"
import { useMemo, useState } from "react"
import type { ReactNode } from "react"
import { Link, useNavigate, useParams } from "react-router"
import { Avatar } from "@/components/avatar"
import {
  Badge,
  Button,
  buttonVariants,
  Card,
  ConfirmDialog,
  CopyButton,
  EmptyState,
  Select,
  Skeleton,
  Spinner,
  StatusBadge,
  Table,
  TBody,
  TD,
  TH,
  THead,
  TR,
} from "@/components/ui"
import { useAddon } from "@/hooks/addons/use-addon"
import { useAddonReadme } from "@/hooks/addons/use-addon-readme"
import { useDeleteAddon } from "@/hooks/addons/use-delete-addon"
import { useDeleteVersion } from "@/hooks/addons/use-delete-version"
import { useSyncAddon } from "@/hooks/addons/use-sync-addon"
import { useMe } from "@/hooks/auth/use-me"
import type { AddonVersion } from "@/lib/types"

export default function AddonDetailPage() {
  const { id = "" } = useParams()
  const { data: addon, isLoading, isError } = useAddon(id)
  const { data: user } = useMe()
  const sync = useSyncAddon(id)
  const deleteVersion = useDeleteVersion(id)
  const deleteAddon = useDeleteAddon()
  const navigate = useNavigate()

  if (isLoading) return <DetailSkeleton />
  if (isError) return <p className="text-danger">Could not load addon.</p>
  if (!addon) return <p>Addon not found.</p>

  const isOwner = user?.id === addon.repo?.owner_id
  const versions = addon.versions ?? []
  const integration = addon.repo?.integration

  return (
    <div className="flex flex-col gap-6">
      <Link
        to="/"
        className="inline-flex w-fit items-center gap-1 text-sm text-muted transition-colors hover:text-fg"
      >
        <ArrowLeft className="size-4" />
        Back
      </Link>

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-semibold">{addon.name}</h1>
            <Badge
              variant={addon.visibility === "public" ? "neutral" : "warning"}
            >
              {addon.visibility}
            </Badge>
          </div>
          <div className="flex items-center gap-2 text-sm text-muted">
            <Avatar hash={addon.repo?.owner?.gravatar_hash} size={20} />
            {addon.repo?.owner?.username ?? "-"}
          </div>
        </div>
        {isOwner && (
          <div className="flex items-center gap-2">
            <Button
              variant="secondary"
              loading={sync.isPending}
              onClick={() => sync.mutate()}
            >
              {!sync.isPending && <RefreshCw className="size-4" />}
              Sync
            </Button>
            <Link
              to={`/addons/${addon.id}/edit`}
              className={buttonVariants({ variant: "secondary" })}
            >
              <Pencil className="size-4" />
              Edit
            </Link>
          </div>
        )}
      </div>

      <InstallSnippet name={addon.name} />

      <ReadmeSection addonId={addon.id} versions={versions} />

      <Card className="p-5">
        <dl className="grid grid-cols-1 gap-x-8 gap-y-4 text-sm sm:grid-cols-2">
          <Detail label="Repository">
            {addon.repo ? (
              <Link
                to={`/repos/${addon.repo.id}`}
                className="break-all text-accent"
              >
                <code>{addon.repo.git_url}</code>
              </Link>
            ) : (
              "-"
            )}
          </Detail>
          <Detail label="Subpath">
            {addon.subpath ? (
              <code>{addon.subpath}</code>
            ) : (
              <span className="text-muted">(repo root)</span>
            )}
          </Detail>
          <Detail label="Default branch">
            <code>{addon.repo?.default_branch ?? "-"}</code>
          </Detail>
          <Detail label="Integration">
            {integration ? (
              <span>
                {integration.provider}
                {integration.account_name
                  ? ` (${integration.account_name})`
                  : ""}
              </span>
            ) : (
              <span className="text-muted">none (anonymous clone)</span>
            )}
          </Detail>
        </dl>
      </Card>

      <RequiresSection versions={versions} />

      <section className="flex flex-col gap-3">
        <h2 className="text-lg font-semibold">
          Versions{" "}
          <span className="font-normal text-muted">({versions.length})</span>
        </h2>
        {versions.length === 0 ? (
          <EmptyState
            icon={Package}
            title="No versions yet"
            description="Sync the addon to discover tags and branches."
          />
        ) : (
          <Table>
            <THead>
              <TR>
                <TH>Version</TH>
                <TH>Ref</TH>
                <TH>Status</TH>
                <TH>Size</TH>
                <TH>Built</TH>
                <TH></TH>
                {isOwner && <TH></TH>}
              </TR>
            </THead>
            <TBody>
              {versions.map((v) => (
                <TR key={v.id}>
                  <TD className="font-medium">
                    <span className="inline-flex items-center gap-2">
                      {v.version}
                      {v.is_latest && <Badge variant="success">latest</Badge>}
                    </span>
                  </TD>
                  <TD className="text-muted">
                    <span className="text-xs">{v.ref_type}</span>{" "}
                    <code className="text-xs">{v.ref_value.slice(0, 8)}</code>
                  </TD>
                  <TD>
                    <StatusBadge status={v.status} />
                  </TD>
                  <TD className="text-muted">{formatSize(v.size_bytes)}</TD>
                  <TD className="text-muted">
                    {v.built_at
                      ? new Date(v.built_at).toLocaleDateString()
                      : "-"}
                  </TD>
                  <TD>
                    {v.status === "ready" ? (
                      <a
                        href={`/api/v1/addons/${addon.id}/versions/${encodeURIComponent(v.version)}/download`}
                        className="inline-flex items-center gap-1 text-accent"
                      >
                        <Download className="size-4" />
                        Download
                      </a>
                    ) : (
                      <span className="text-muted">-</span>
                    )}
                  </TD>
                  {isOwner && (
                    <TD className="text-right">
                      <ConfirmDialog
                        trigger={
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-label={`Delete version ${v.version}`}
                          >
                            <Trash2 className="size-4" />
                          </Button>
                        }
                        title={`Delete version ${v.version}?`}
                        description="This removes the version and its built zipball."
                        confirmLabel="Delete"
                        destructive
                        loading={deleteVersion.isPending}
                        onConfirm={() => deleteVersion.mutate(v.version)}
                      />
                    </TD>
                  )}
                </TR>
              ))}
            </TBody>
          </Table>
        )}
      </section>

      {isOwner && (
        <Card className="border-danger/30 p-5">
          <h2 className="text-base font-semibold text-danger">Danger zone</h2>
          <p className="mt-1 text-sm text-muted">
            Removes the addon, all its versions, and built zipballs. The
            underlying repo stays.
          </p>
          <div className="mt-4">
            <ConfirmDialog
              trigger={
                <Button variant="danger" loading={deleteAddon.isPending}>
                  <Trash2 className="size-4" />
                  Delete addon
                </Button>
              }
              title={`Delete addon ${addon.name}?`}
              description="This cannot be undone."
              confirmLabel="Delete addon"
              destructive
              loading={deleteAddon.isPending}
              onConfirm={() =>
                deleteAddon.mutate(addon.id, {
                  onSuccess: () => navigate("/"),
                })
              }
            />
          </div>
          {deleteAddon.isError && (
            <p className="mt-2 text-sm text-danger">
              Delete failed: {deleteAddon.error.message}
            </p>
          )}
        </Card>
      )}
    </div>
  )
}

function RequiresSection({ versions }: { versions: AddonVersion[] }) {
  const target =
    versions.find((v) => v.is_latest) ??
    versions.find((v) => v.status === "ready") ??
    versions[0]
  const deps = target?.depends_resolved ?? []
  if (deps.length === 0) return null

  return (
    <Card>
      <div className="flex items-center gap-2 border-b border-border p-4 font-medium">
        <Boxes className="size-4 text-muted" />
        Requires
        <span className="font-normal text-muted">({deps.length})</span>
        {target?.version && (
          <span className="ml-auto text-xs font-normal text-muted">
            for {target.version}
          </span>
        )}
      </div>
      <ul className="divide-y divide-border">
        {deps.map((dep) => (
          <li
            key={dep.module}
            className="flex items-center justify-between gap-3 px-4 py-2.5 text-sm"
          >
            {dep.addon_id ? (
              <Link
                to={`/addons/${dep.addon_id}`}
                className="font-mono text-accent"
              >
                {dep.module}
              </Link>
            ) : (
              <code className="font-mono text-muted">{dep.module}</code>
            )}
            {dep.access === "external" && (
              <Badge variant="neutral">external</Badge>
            )}
            {dep.access === "forbidden" && (
              <Badge variant="warning">private</Badge>
            )}
          </li>
        ))}
      </ul>
    </Card>
  )
}

function Detail({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-1">
      <dt className="text-xs font-medium uppercase tracking-wide text-muted">
        {label}
      </dt>
      <dd>{children}</dd>
    </div>
  )
}

function InstallSnippet({ name }: { name: string }) {
  const cmd = `odoopack add ${name}`
  return (
    <div className="flex items-center gap-2 rounded-lg border border-border bg-surface px-3 py-1.5 font-mono text-sm">
      <span className="select-none text-muted">$</span>
      <code className="flex-1 truncate">{cmd}</code>
      <CopyButton value={cmd} />
    </div>
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
  const [selected, setSelected] = useState("")
  const defaultVersion =
    readable.find((v) => v.is_latest)?.version ?? readable[0]?.version ?? ""
  const version = selected || defaultVersion
  const { data, isLoading, isError } = useAddonReadme(addonId, version)

  if (readable.length === 0) return null

  return (
    <Card>
      <div className="flex items-center justify-between gap-3 border-b border-border p-4">
        <div className="flex items-center gap-2 font-medium">
          <FileText className="size-4 text-muted" />
          README
        </div>
        <Select
          value={version}
          onChange={(e) => setSelected(e.target.value)}
          className="h-8 w-auto py-1 text-sm"
        >
          {readable.map((v) => (
            <option key={v.id} value={v.version}>
              {v.version}
            </option>
          ))}
        </Select>
      </div>
      <div className="p-5">
        {isLoading ? (
          <div className="flex justify-center py-6">
            <Spinner />
          </div>
        ) : isError ? (
          <p className="text-sm text-danger">Could not load README.</p>
        ) : !data ? (
          <p className="text-sm text-muted">No README for this version.</p>
        ) : (
          <div
            className="prose prose-sm max-w-none dark:prose-invert"
            dangerouslySetInnerHTML={{ __html: data.html }}
          />
        )}
      </div>
    </Card>
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

function DetailSkeleton() {
  return (
    <div className="flex flex-col gap-6">
      <Skeleton className="h-4 w-16" />
      <div className="flex flex-col gap-2">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-4 w-40" />
      </div>
      <Skeleton className="h-10 w-full" />
      <Skeleton className="h-48 w-full rounded-xl" />
      <Skeleton className="h-32 w-full rounded-xl" />
    </div>
  )
}

function formatSize(bytes?: number): string {
  if (!bytes) return "-"
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}
