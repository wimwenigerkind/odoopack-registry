import { useState } from "react"
import { Link } from "react-router"
import { useMe } from "@/hooks/auth/use-me"
import { useProviders } from "@/hooks/auth/use-providers"
import { useUnlinkIdentity } from "@/hooks/auth/use-unlink-identity"
import { Avatar } from "@/components/avatar"
import {
  useCreateToken,
  useDeleteToken,
  useTokens,
} from "@/hooks/tokens/use-tokens"
import {
  useDeleteIntegration,
  useIntegrationProviders,
  useIntegrations,
} from "@/hooks/integrations/use-integrations"
import { BACKEND_BASE } from "@/lib/api"

export default function ProfilePage() {
  const { data: user, isLoading, isError } = useMe()

  if (isLoading) return <p>Loading...</p>
  if (isError) return <p>Could not load profile.</p>
  if (!user) return <p>You must be logged in.</p>

  const identities = user.identities ?? []

  return (
    <>
      <p>
        <Link to="/">Back</Link>
      </p>

      <h2>Profile</h2>

      <Avatar email={user.email} size={64} />

      <dl>
        <dt>Username</dt>
        <dd>{user.username || "-"}</dd>
        <dt>Email</dt>
        <dd>{user.email}</dd>
        <dt>User ID</dt>
        <dd>
          <code>{user.id}</code>
        </dd>
        <dt>Member since</dt>
        <dd>{new Date(user.created_at).toLocaleDateString()}</dd>
      </dl>

      <ConnectedAccounts identities={identities} />

      <LinkAccounts />

      <Tokens />

      <Integrations />
    </>
  )
}

function ConnectedAccounts({ identities }: { identities: { id: string; provider: string; created_at: string }[] }) {
  const unlink = useUnlinkIdentity()
  return (
    <>
      <h3>Connected accounts</h3>
      {identities.length === 0 ? (
        <p>No connected accounts.</p>
      ) : (
        <ul>
          {identities.map((i) => (
            <li key={i.id}>
              <strong>{i.provider}</strong>{" "}
              <small>linked {new Date(i.created_at).toLocaleDateString()}</small>{" "}
              <button
                onClick={() => {
                  if (confirm(`Unlink ${i.provider} account?`)) {
                    unlink.mutate(i.id)
                  }
                }}
                disabled={unlink.isPending}
              >
                Unlink
              </button>
            </li>
          ))}
        </ul>
      )}
      {unlink.isError && <p>Unlink failed: {unlink.error.message}</p>}
    </>
  )
}

function LinkAccounts() {
  const { data: providersData, isLoading } = useProviders()
  const providers = providersData?.providers ?? []

  const linkHref = (provider: string) =>
    `${BACKEND_BASE}/auth/${provider}/link?return_to=${encodeURIComponent("/profile")}`

  if (isLoading) return null
  if (providers.length === 0) return null

  return (
    <>
      <h3>Link another account</h3>
      <ul>
        {providers.map((p) => (
          <li key={p.name}>
            <a href={linkHref(p.name)}>Link {p.name}</a>
          </li>
        ))}
      </ul>
    </>
  )
}

function Integrations() {
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
    <>
      <h3>Git Integrations</h3>
      <p>
        Connect a git provider so the registry can clone your private repos
        when syncing addons.
      </p>

      {providersLoading ? (
        <p>Loading providers...</p>
      ) : providersError ? (
        <p>Could not load providers: {providersErr?.message}</p>
      ) : providers.length === 0 ? (
        <p>No integration providers configured on this server.</p>
      ) : (
        <ul>
          {providers.map((p) => (
            <li key={p.name}>
              <a href={connectHref(p.name)}>Connect {p.name}</a>
            </li>
          ))}
        </ul>
      )}

      {isLoading && <p>Loading integrations...</p>}
      {isError && <p>Could not load integrations.</p>}
      {integrations && integrations.length === 0 && (
        <p>No integrations connected.</p>
      )}
      {integrations && integrations.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>Provider</th>
              <th>Account</th>
              <th>Connected</th>
              <th>Webhook URL</th>
              <th>Webhook Secret</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {integrations.map((i) => {
              const url = `${window.location.origin}/webhooks/${i.provider}/${i.id}`
              return (
                <tr key={i.id}>
                  <td>{i.provider}</td>
                  <td>{i.account_name || "-"}</td>
                  <td>{new Date(i.created_at).toLocaleDateString()}</td>
                  <td>
                    <code>{url}</code>{" "}
                    <button
                      type="button"
                      onClick={() => navigator.clipboard?.writeText(url)}
                    >
                      Copy
                    </button>
                  </td>
                  <td>
                    <code>{i.hook_secret}</code>{" "}
                    <button
                      type="button"
                      onClick={() => navigator.clipboard?.writeText(i.hook_secret)}
                    >
                      Copy
                    </button>
                  </td>
                  <td>
                    <button
                      onClick={() => {
                        if (confirm(`Disconnect ${i.provider} (${i.account_name})?`)) {
                          deleteIntegration.mutate(i.id)
                        }
                      }}
                      disabled={deleteIntegration.isPending}
                    >
                      Disconnect
                    </button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      )}
    </>
  )
}

function Tokens() {
  const { data: tokens, isLoading, isError } = useTokens()
  const createToken = useCreateToken()
  const deleteToken = useDeleteToken()
  const [name, setName] = useState("")

  const submit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    createToken.mutate(
      { name: name.trim() },
      {
        onSuccess: () => {
          setName("")
        },
      },
    )
  }

  return (
    <>
      <h3>API Tokens</h3>
      <p>
        Used by the CLI to access this registry. Send as{" "}
        <code>Authorization: Bearer &lt;token&gt;</code>.
      </p>

      <form onSubmit={submit}>
        <label>
          Name{" "}
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="my laptop"
            required
          />
        </label>{" "}
        <button type="submit" disabled={createToken.isPending}>
          {createToken.isPending ? "Creating..." : "Create token"}
        </button>
      </form>
      {createToken.isError && (
        <p>Error: {createToken.error.message}</p>
      )}

      {isLoading && <p>Loading tokens...</p>}
      {isError && <p>Could not load tokens.</p>}
      {tokens && tokens.length === 0 && <p>No tokens yet.</p>}
      {tokens && tokens.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Token</th>
              <th>Created</th>
              <th>Last used</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {tokens.map((t) => (
              <tr key={t.id}>
                <td>{t.name}</td>
                <td>
                  <code>{t.token}</code>
                </td>
                <td>{new Date(t.created_at).toLocaleDateString()}</td>
                <td>
                  {t.last_used_at
                    ? new Date(t.last_used_at).toLocaleString()
                    : "never"}
                </td>
                <td>
                  <button
                    onClick={() => {
                      if (confirm(`Revoke token "${t.name}"?`)) {
                        deleteToken.mutate(t.id)
                      }
                    }}
                    disabled={deleteToken.isPending}
                  >
                    Revoke
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  )
}
