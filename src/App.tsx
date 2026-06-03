import './App.css'
import { useMe } from "@/hooks/auth/use-me"
import { useProviders } from "@/hooks/auth/use-providers"
import { useLogout } from "@/hooks/auth/use-logout"
import { useAddons } from "@/hooks/addons/use-addons"

function App() {
  const { data: user, isLoading: meLoading, isError: meError } = useMe()
  const { data: providersData, isLoading: providersLoading } = useProviders()
  const { data: addonsData, isLoading: addonsLoading } = useAddons()
  const logout = useLogout()

  const providers = providersData?.providers ?? []
  const addons = addonsData ?? []
  const displayName = user?.username || user?.identities?.[0]?.email || ""

  const loginHref = (name: string) =>
    `/auth/${name}/login?return_to=${encodeURIComponent(
      window.location.pathname + window.location.search
    )}`

  return (
    <>
      <h1>Odoopack Registry</h1>

      {meLoading ? (
        <p>Loading…</p>
      ) : meError ? (
        <p>Could not check login state. Try reloading.</p>
      ) : user ? (
        <section>
          <h2>Logged in as {displayName}</h2>
          <button
            onClick={() => logout.mutate()}
            disabled={logout.isPending}
          >
            {logout.isPending ? "Logging out…" : "Logout"}
          </button>
        </section>
      ) : (
        <section>
          <h2>Not logged in</h2>
          {providersLoading ? (
            <p>Loading login options…</p>
          ) : providers.length === 0 ? (
            <p>No login providers configured.</p>
          ) : (
            <ul>
              {providers.map((p) => (
                <li key={p.name}>
                  <a href={loginHref(p.name)}>Login with {p.name}</a>
                </li>
              ))}
            </ul>
          )}
        </section>
      )}

      <section>
        <h2>Addons</h2>
        {addonsLoading ? (
          <p>Loading addons…</p>
        ) : addons.length === 0 ? (
          <p>No addons visible.</p>
        ) : (
          <ul>
            {addons.map((a) => (
              <li key={a.id}>
                <span>{a.name}</span>{" "}
                <small>({a.visibility})</small>{" "}
                <small>{a.versions?.length ?? 0} versions</small>
              </li>
            ))}
          </ul>
        )}
      </section>
    </>
  )
}

export default App
