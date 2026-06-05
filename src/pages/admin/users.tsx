import { Link } from "react-router"
import { useUsers } from "@/hooks/admin/use-users"

export default function AdminUsersPage() {
  const { data: users, isLoading, isError } = useUsers()

  return (
    <>
      <p>
        <Link to="/">Back</Link>
      </p>
      <h2>Users</h2>
      {isLoading ? (
        <p>Loading...</p>
      ) : isError ? (
        <p>Could not load users</p>
      ) : (
        <table>
          <thead>
            <tr>
              <th>Email</th>
              <th>Username</th>
              <th>Admin</th>
              <th>ID</th>
            </tr>
          </thead>
          <tbody>
            {(users ?? []).map((u) => (
              <tr key={u.id}>
                <td>{u.email}</td>
                <td>{u.username || "-"}</td>
                <td>{u.is_admin ? "yes" : "no"}</td>
                <td>
                  <code>{u.id}</code>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  )
}
