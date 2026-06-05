import { Link } from "react-router"
import { useAddons } from "@/hooks/addons/use-addons"
import { useMe } from "@/hooks/auth/use-me"

export default function HomePage() {
  const { data: addonsData, isLoading: addonsLoading } = useAddons()
  const { data: user } = useMe()

  const addons = addonsData ?? []

  return (
    <>
      <section>
        <h2>Addons</h2>
        {user && (
          <p>
            <Link to="/addons/new">Register addon</Link>
          </p>
        )}
        {addonsLoading ? (
          <p>Loading addons…</p>
        ) : addons.length === 0 ? (
          user ? (
            <p>
              No addons yet. <Link to="/addons/new">Register your first addon</Link>.
            </p>
          ) : (
            <p>No addons visible.</p>
          )
        ) : (
          <ul>
            {addons.map((a) => (
              <li key={a.id}>
                <Link to={`/addons/${a.id}`}>{a.name}</Link>{" "}
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
