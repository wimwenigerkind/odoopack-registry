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
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Visibility</th>
                <th>Versions</th>
                <th>Updated</th>
              </tr>
            </thead>
            <tbody>
              {addons.map((a) => (
                <tr key={a.id}>
                  <td>
                    <Link to={`/addons/${a.id}`}>{a.name}</Link>
                  </td>
                  <td>{a.visibility}</td>
                  <td>{a.versions?.length ?? 0}</td>
                  <td>{new Date(a.updated_at).toLocaleDateString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </>
  )
}
