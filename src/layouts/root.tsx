import {Link, Outlet} from "react-router";
import {useMe} from "@/hooks/auth/use-me.ts";
import {useProviders} from "@/hooks/auth/use-providers.ts";
import {useLogout} from "@/hooks/auth/use-logout.ts";
import {Avatar} from "@/components/avatar";

export default function RootLayout() {
  const {data: user, isLoading: meLoading, isError: meError} = useMe()
  const {data: providersData, isLoading: providersLoading} = useProviders()
  const logout = useLogout()

  const providers = providersData?.providers ?? []
  const displayName = user?.username || user?.email || ""

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
          <h2>
            <Link to="/profile">
              <Avatar email={user.email} size={32} />
              {" "}{displayName}
            </Link>
          </h2>
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
      <Outlet/>
    </>
  )
}