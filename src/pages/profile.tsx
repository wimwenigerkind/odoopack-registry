import {
  FolderGit2,
  KeyRound,
  Link2,
  Plug,
  Plus,
  Trash2,
} from "lucide-react"
import type { ComponentType } from "react"
import { useState } from "react"
import type { FormEvent } from "react"
import { Link } from "react-router"
import { Avatar } from "@/components/avatar"
import {
  Badge,
  Button,
  buttonVariants,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
  ConfirmDialog,
  CopyButton,
  EmptyState,
  Field,
  Input,
  SecretValue,
  Spinner,
  Table,
  TBody,
  TD,
  TH,
  THead,
  TR,
} from "@/components/ui"
import { useMe } from "@/hooks/auth/use-me"
import { useProviders } from "@/hooks/auth/use-providers"
import { useUnlinkIdentity } from "@/hooks/auth/use-unlink-identity"
import { useCreateWorkspaceHook } from "@/hooks/integrations/use-create-workspace-hook"
import {
  useDeleteIntegration,
  useIntegrationProviders,
  useIntegrations,
} from "@/hooks/integrations/use-integrations"
import { useMyRepos } from "@/hooks/repos/use-my-repos"
import {
  useCreateToken,
  useDeleteToken,
  useTokens,
} from "@/hooks/tokens/use-tokens"
import { BACKEND_BASE } from "@/lib/api"
import { cn } from "@/lib/cn"
import type { OAuthIntegration } from "@/lib/types"

type Identity = { id: string; provider: string; created_at: string }

export default function ProfilePage() {
  const { data: user, isLoading, isError } = useMe()
  const { data: repos } = useMyRepos()
  const { data: tokens } = useTokens()
  const { data: integrations } = useIntegrations()

  if (isLoading)
    return (
      <div className="flex justify-center py-16">
        <Spinner className="size-6" />
      </div>
    )
  if (isError) return <p className="text-destructive">Could not load profile.</p>
  if (!user) return <p>You must be logged in.</p>

  const identities = user.identities ?? []
  const stats: StatItem[] = [
    { label: "Repositories", value: repos?.length ?? 0, icon: FolderGit2 },
    { label: "API tokens", value: tokens?.length ?? 0, icon: KeyRound },
    { label: "Integrations", value: integrations?.length ?? 0, icon: Plug },
    { label: "Accounts", value: identities.length, icon: Link2 },
  ]

  return (
    <div className="flex flex-col gap-6">
      <Card className="overflow-hidden p-0">
        <div className="h-24 bg-gradient-to-r from-primary/25 via-primary/10 to-transparent" />
        <div className="-mt-10 flex flex-wrap items-end gap-4 px-6 pb-6">
          <div className="overflow-hidden rounded-2xl bg-card ring-4 ring-card">
            <Avatar hash={user.gravatar_hash} size={80} />
          </div>
          <div className="min-w-0 flex-1 pb-1">
            <h1 className="truncate text-2xl font-semibold tracking-tight">
              {user.username || user.email}
            </h1>
            <p className="truncate text-sm text-muted-foreground">{user.email}</p>
          </div>
          <div className="flex flex-wrap items-center gap-2 pb-1">
            {user.is_admin && <Badge variant="accent">admin</Badge>}
            <Badge variant="neutral">
              Since {new Date(user.created_at).toLocaleDateString()}
            </Badge>
          </div>
        </div>
      </Card>

      <Card className="divide-y divide-border sm:flex sm:divide-x sm:divide-y-0">
        {stats.map((s) => (
          <StatCell key={s.label} {...s} />
        ))}
      </Card>

      <MyReposCard />
      <TokensCard />
      <AccountsCard identities={identities} />
      <IntegrationsCard />
    </div>
  )
}

type StatItem = {
  label: string
  value: number
  icon: ComponentType<{ className?: string }>
}

function StatCell({ label, value, icon: Icon }: StatItem) {
  return (
    <div className="flex flex-1 items-center gap-3 p-5">
      <span className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
        <Icon className="size-5" />
      </span>
      <div className="min-w-0">
        <div className="text-2xl font-semibold leading-none tabular-nums">
          {value}
        </div>
        <div className="mt-1 truncate text-xs text-muted-foreground">{label}</div>
      </div>
    </div>
  )
}

function AccountsCard({ identities }: { identities: Identity[] }) {
  const unlink = useUnlinkIdentity()
  const { data: providersData } = useProviders()
  const providers = providersData?.providers ?? []
  const linkHref = (provider: string) =>
    `${BACKEND_BASE}/auth/${provider}/link?return_to=${encodeURIComponent("/profile")}`

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Link2 className="size-4 text-muted-foreground" />
          Connected accounts
        </CardTitle>
        <CardDescription>
          Sign-in providers linked to your account.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {identities.length === 0 ? (
          <p className="text-sm text-muted-foreground">No connected accounts.</p>
        ) : (
          <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border">
            {identities.map((i) => (
              <li
                key={i.id}
                className="flex items-center justify-between gap-2 px-4 py-2.5 text-sm"
              >
                <span className="flex items-center gap-2">
                  <span className="font-medium capitalize">{i.provider}</span>
                  <span className="text-xs text-muted-foreground">
                    linked {new Date(i.created_at).toLocaleDateString()}
                  </span>
                </span>
                <ConfirmDialog
                  trigger={
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-destructive hover:bg-destructive/10 hover:text-destructive"
                    >
                      Unlink
                    </Button>
                  }
                  title={`Unlink ${i.provider}?`}
                  confirmLabel="Unlink"
                  destructive
                  loading={unlink.isPending}
                  onConfirm={() => unlink.mutate(i.id)}
                />
              </li>
            ))}
          </ul>
        )}
        {unlink.isError && (
          <p className="text-sm text-destructive">
            Unlink failed: {unlink.error.message}
          </p>
        )}
        {providers.length > 0 && (
          <div className="flex flex-wrap items-center gap-2 border-t border-border pt-4">
            <span className="text-sm text-muted-foreground">Link another:</span>
            {providers.map((p) => (
              <a
                key={p.name}
                href={linkHref(p.name)}
                className={cn(
                  buttonVariants({ variant: "secondary", size: "sm" }),
                  "capitalize",
                )}
              >
                {p.name}
              </a>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function MyReposCard() {
  const { data: repos, isLoading, isError } = useMyRepos()
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <FolderGit2 className="size-4 text-muted-foreground" />
          My repositories
        </CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="flex justify-center py-4">
            <Spinner />
          </div>
        ) : isError ? (
          <p className="text-sm text-destructive">Could not load repos.</p>
        ) : !repos || repos.length === 0 ? (
          <EmptyState
            title="No repositories"
            description="You don't own any repos yet. Register an addon to create one."
            action={
              <Link
                to="/addons/new"
                className={buttonVariants({ size: "sm" })}
              >
                Register an addon
              </Link>
            }
          />
        ) : (
          <Table>
            <THead>
              <TR>
                <TH>Git URL</TH>
                <TH>Default branch</TH>
                <TH>Integration</TH>
                <TH>Registered</TH>
              </TR>
            </THead>
            <TBody>
              {repos.map((r) => (
                <TR key={r.id}>
                  <TD className="font-medium">
                    <Link
                      to={`/repos/${r.id}`}
                      className="break-all hover:text-primary"
                    >
                      <code>{r.git_url}</code>
                    </Link>
                  </TD>
                  <TD className="text-muted-foreground">
                    <code>{r.default_branch}</code>
                  </TD>
                  <TD className="text-muted-foreground">
                    {r.integration
                      ? `${r.integration.provider}${r.integration.account_name ? ` (${r.integration.account_name})` : ""}`
                      : "-"}
                  </TD>
                  <TD className="text-muted-foreground">
                    {new Date(r.created_at).toLocaleDateString()}
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}

function TokensCard() {
  const { data: tokens, isLoading, isError } = useTokens()
  const createToken = useCreateToken()
  const deleteToken = useDeleteToken()
  const [name, setName] = useState("")

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    createToken.mutate(
      { name: name.trim() },
      { onSuccess: () => setName("") },
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <KeyRound className="size-4 text-muted-foreground" />
          API tokens
        </CardTitle>
        <CardDescription>
          Used by the CLI to access this registry. Send as{" "}
          <code>Authorization: Bearer &lt;token&gt;</code>.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <form onSubmit={submit} className="flex items-end gap-2">
          <div className="flex-1">
            <Field label="Token name" htmlFor="token-name">
              <Input
                id="token-name"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="my laptop"
              />
            </Field>
          </div>
          <Button type="submit" loading={createToken.isPending}>
            <Plus className="size-4" />
            Create
          </Button>
        </form>
        {createToken.isError && (
          <p className="text-sm text-destructive">
            Error: {createToken.error.message}
          </p>
        )}

        {isLoading ? (
          <div className="flex justify-center py-4">
            <Spinner />
          </div>
        ) : isError ? (
          <p className="text-sm text-destructive">Could not load tokens.</p>
        ) : !tokens || tokens.length === 0 ? (
          <p className="text-sm text-muted-foreground">No tokens yet.</p>
        ) : (
          <Table>
            <THead>
              <TR>
                <TH>Name</TH>
                <TH>Token</TH>
                <TH>Created</TH>
                <TH>Last used</TH>
                <TH></TH>
              </TR>
            </THead>
            <TBody>
              {tokens.map((t) => (
                <TR key={t.id}>
                  <TD className="font-medium">{t.name}</TD>
                  <TD>
                    <SecretValue value={t.token} />
                  </TD>
                  <TD className="text-muted-foreground">
                    {new Date(t.created_at).toLocaleDateString()}
                  </TD>
                  <TD className="text-muted-foreground">
                    {t.last_used_at
                      ? new Date(t.last_used_at).toLocaleString()
                      : "never"}
                  </TD>
                  <TD className="text-right">
                    <ConfirmDialog
                      trigger={
                        <Button
                          variant="ghost"
                          size="sm"
                          aria-label={`Revoke token ${t.name}`}
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      }
                      title={`Revoke token ${t.name}?`}
                      confirmLabel="Revoke"
                      destructive
                      loading={deleteToken.isPending}
                      onConfirm={() => deleteToken.mutate(t.id)}
                    />
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}

function IntegrationsCard() {
  const { data: integrations, isLoading, isError } = useIntegrations()
  const {
    data: providersData,
    isLoading: providersLoading,
    isError: providersError,
    error: providersErr,
  } = useIntegrationProviders()
  const deleteIntegration = useDeleteIntegration()

  const providers = providersData?.providers ?? []
  const connectHref = (provider: string) =>
    `${BACKEND_BASE}/integrations/${provider}/connect?return_to=${encodeURIComponent("/profile")}`

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Plug className="size-4 text-muted-foreground" />
          Git integrations
        </CardTitle>
        <CardDescription>
          Connect a git provider so the registry can clone your private repos
          when syncing addons.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {providersLoading ? (
          <div className="flex justify-center py-2">
            <Spinner />
          </div>
        ) : providersError ? (
          <p className="text-sm text-destructive">
            Could not load providers: {providersErr?.message}
          </p>
        ) : providers.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No integration providers configured on this server.
          </p>
        ) : (
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm text-muted-foreground">Connect:</span>
            {providers.map((p) => (
              <a
                key={p.name}
                href={connectHref(p.name)}
                className={cn(
                  buttonVariants({ variant: "secondary", size: "sm" }),
                  "capitalize",
                )}
              >
                {p.name}
              </a>
            ))}
          </div>
        )}

        {isLoading ? (
          <div className="flex justify-center py-4">
            <Spinner />
          </div>
        ) : isError ? (
          <p className="text-sm text-destructive">Could not load integrations.</p>
        ) : !integrations || integrations.length === 0 ? (
          <p className="text-sm text-muted-foreground">No integrations connected.</p>
        ) : (
          <div className="flex flex-col gap-3">
            {integrations.map((i) => (
              <IntegrationPanel
                key={i.id}
                integration={i}
                onDisconnect={() => deleteIntegration.mutate(i.id)}
                disconnectPending={deleteIntegration.isPending}
              />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function IntegrationPanel({
  integration: i,
  onDisconnect,
  disconnectPending,
}: {
  integration: OAuthIntegration
  onDisconnect: () => void
  disconnectPending: boolean
}) {
  const url = `${window.location.origin}/webhooks/${i.provider}/${i.id}`
  const createHook = useCreateWorkspaceHook(i.id)
  const [workspace, setWorkspace] = useState("")

  const submitHook = (e: FormEvent) => {
    e.preventDefault()
    if (!workspace.trim()) return
    createHook.mutate(workspace.trim(), { onSuccess: () => setWorkspace("") })
  }

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border p-4">
      <div className="flex items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-medium capitalize">{i.provider}</span>
          {i.account_name && <Badge variant="accent">{i.account_name}</Badge>}
          <span className="text-xs text-muted-foreground">
            connected {new Date(i.created_at).toLocaleDateString()}
          </span>
        </div>
        <ConfirmDialog
          trigger={
            <Button variant="ghost" size="sm">
              <Trash2 className="size-4" />
              Disconnect
            </Button>
          }
          title={`Disconnect ${i.provider}?`}
          description={i.account_name || undefined}
          confirmLabel="Disconnect"
          destructive
          loading={disconnectPending}
          onConfirm={onDisconnect}
        />
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <Field label="Webhook URL">
          <div className="flex items-center gap-1">
            <code className="flex-1 truncate rounded-md bg-foreground/5 px-2 py-1 text-xs">
              {url}
            </code>
            <CopyButton value={url} />
          </div>
        </Field>
        <Field label="Webhook secret">
          <SecretValue value={i.hook_secret ?? ""} />
        </Field>
      </div>

      {i.provider === "bitbucket" && (
        <form
          onSubmit={submitHook}
          className="flex flex-col gap-2 border-t border-border pt-3"
        >
          <span className="text-sm font-medium">Workspace hook</span>
          <div className="flex gap-2">
            <Input
              value={workspace}
              onChange={(e) => setWorkspace(e.target.value)}
              placeholder="workspace-slug"
              className="h-9"
            />
            <Button type="submit" size="sm" loading={createHook.isPending}>
              Create
            </Button>
          </div>
          {createHook.isSuccess && (
            <p className="text-xs text-success">
              Created hook {createHook.data.hook_uuid}
            </p>
          )}
          {createHook.isError && (
            <p className="text-xs text-destructive">{createHook.error.message}</p>
          )}
        </form>
      )}
    </div>
  )
}
