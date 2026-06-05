import { Link } from "react-router"
import { useMe } from "@/hooks/auth/use-me"
import { Avatar } from "@/components/avatar"

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

      <h3>Connected accounts</h3>
      {identities.length === 0 ? (
        <p>No connected accounts.</p>
      ) : (
        <ul>
          {identities.map((i) => (
            <li key={i.id}>
              <strong>{i.provider}</strong>{" "}
              <small>
                linked {new Date(i.created_at).toLocaleDateString()}
              </small>
            </li>
          ))}
        </ul>
      )}
    </>
  )
}
