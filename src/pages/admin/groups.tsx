import { type FormEvent, useState } from "react"
import { Link } from "react-router"
import {
  useCreateGroup,
  useDeleteGroup,
  useGroups,
} from "@/hooks/admin/use-groups"

export default function AdminGroupsPage() {
  const { data: groups, isLoading, isError } = useGroups()
  const createGroup = useCreateGroup()
  const deleteGroup = useDeleteGroup()
  const [name, setName] = useState("")

  const handleCreate = (e: FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    createGroup.mutate(
      { name: name.trim() },
      {
        onSuccess: () => setName(""),
      },
    )
  }

  return (
    <>
      <p>
        <Link to="/">Back</Link>
      </p>
      <h2>Groups</h2>

      <form onSubmit={handleCreate}>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Group name"
        />{" "}
        <button type="submit" disabled={createGroup.isPending}>
          {createGroup.isPending ? "Creating..." : "Create"}
        </button>
        {createGroup.isError && (
          <p>Failed to create: {createGroup.error.message}</p>
        )}
      </form>

      {isLoading ? (
        <p>Loading...</p>
      ) : isError ? (
        <p>Could not load groups</p>
      ) : (groups ?? []).length === 0 ? (
        <p>No groups yet.</p>
      ) : (
        <ul>
          {(groups ?? []).map((g) => (
            <li key={g.id}>
              <Link to={`/admin/groups/${g.id}`}>{g.name}</Link>{" "}
              <button
                onClick={() => {
                  if (confirm(`Delete group "${g.name}"?`)) {
                    deleteGroup.mutate(g.id)
                  }
                }}
                disabled={deleteGroup.isPending}
              >
                Delete
              </button>
            </li>
          ))}
        </ul>
      )}
    </>
  )
}
