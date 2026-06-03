import { Link } from "react-router"
import { useAddons } from "@/hooks/addons/use-addons"

export default function HomePage() {
  const { data: addonsData, isLoading: addonsLoading } = useAddons()

  const addons = addonsData ?? []

  return (
    <>
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
