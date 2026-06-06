import { type FormEvent, useMemo, useState } from "react"
import { Link, useParams } from "react-router"
import { useGroup } from "@/hooks/admin/use-group"
import {
  useAddGroupMember,
  useGroupMembers,
  useRemoveGroupMember,
} from "@/hooks/admin/use-group-members"
import {
  useGrantGroupAddon,
  useGroupAddons,
  useRevokeGroupAddon,
} from "@/hooks/admin/use-group-addons"
import { useUsers } from "@/hooks/admin/use-users"
import { useAddons } from "@/hooks/addons/use-addons"

export default function AdminGroupDetailPage() {
  const { id = "" } = useParams()
  const { data: group, isLoading, isError } = useGroup(id)
  const { data: members } = useGroupMembers(id)
  const { data: addonAccess } = useGroupAddons(id)
  const { data: users } = useUsers()
  const { data: addons } = useAddons()
  const addMember = useAddGroupMember(id)
  const removeMember = useRemoveGroupMember(id)
  const grantAddon = useGrantGroupAddon(id)
  const revokeAddon = useRevokeGroupAddon(id)

  const [memberToAdd, setMemberToAdd] = useState("")
  const [addonToAdd, setAddonToAdd] = useState("")

  const usersByID = useMemo(() => {
    const m = new Map<string, (typeof users extends readonly (infer U)[] | undefined ? U : never)>()
    ;(users ?? []).forEach((u) => m.set(u.id, u))
    return m
  }, [users])

  const addonsByID = useMemo(() => {
    const m = new Map<string, (typeof addons extends readonly (infer A)[] | undefined ? A : never)>()
    ;(addons ?? []).forEach((a) => m.set(a.id, a))
    return m
  }, [addons])

  if (isLoading) return <p>Loading...</p>
  if (isError) return <p>Could not load group</p>
  if (!group) return <p>Group not found</p>

  const memberIDs = new Set((members ?? []).map((m) => m.user_id))
  const accessAddonIDs = new Set((addonAccess ?? []).map((a) => a.addon_id))

  const availableUsers = (users ?? []).filter((u) => !memberIDs.has(u.id))
  const availableAddons = (addons ?? []).filter((a) => !accessAddonIDs.has(a.id))

  const handleAddMember = (e: FormEvent) => {
    e.preventDefault()
    if (!memberToAdd) return
    addMember.mutate(
      { user_id: memberToAdd },
      { onSuccess: () => setMemberToAdd("") },
    )
  }

  const handleGrantAddon = (e: FormEvent) => {
    e.preventDefault()
    if (!addonToAdd) return
    grantAddon.mutate(
      { addon_id: addonToAdd },
      { onSuccess: () => setAddonToAdd("") },
    )
  }

  return (
    <>
      <p>
        <Link to="/admin/groups">Back</Link>
      </p>
      <h2>Group: {group.name}</h2>

      <section>
        <h3>Members ({members?.length ?? 0})</h3>
        <form onSubmit={handleAddMember}>
          <select
            value={memberToAdd}
            onChange={(e) => setMemberToAdd(e.target.value)}
          >
            <option value="">Select a user</option>
            {availableUsers.map((u) => (
              <option key={u.id} value={u.id}>
                {u.email} {u.username ? `(${u.username})` : ""}
              </option>
            ))}
          </select>{" "}
          <button
            type="submit"
            disabled={!memberToAdd || addMember.isPending}
          >
            Add
          </button>
        </form>
        {(members ?? []).length === 0 ? (
          <p>No members.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>User</th>
                <th>Joined</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {(members ?? []).map((m) => {
                const u = usersByID.get(m.user_id)
                return (
                  <tr key={m.user_id}>
                    <td>
                      {u ? (
                        <>
                          {u.email}
                          {u.username && <> ({u.username})</>}
                        </>
                      ) : (
                        <code>{m.user_id}</code>
                      )}
                    </td>
                    <td>{new Date(m.created_at).toLocaleDateString()}</td>
                    <td>
                      <button
                        onClick={() => removeMember.mutate(m.user_id)}
                        disabled={removeMember.isPending}
                      >
                        Remove
                      </button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </section>

      <section>
        <h3>Addon access ({addonAccess?.length ?? 0})</h3>
        <form onSubmit={handleGrantAddon}>
          <select
            value={addonToAdd}
            onChange={(e) => setAddonToAdd(e.target.value)}
          >
            <option value="">Select an addon</option>
            {availableAddons.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name} ({a.visibility})
              </option>
            ))}
          </select>{" "}
          <button
            type="submit"
            disabled={!addonToAdd || grantAddon.isPending}
          >
            Grant
          </button>
        </form>
        {(addonAccess ?? []).length === 0 ? (
          <p>No addon access granted.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Addon</th>
                <th>Visibility</th>
                <th>Granted</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {(addonAccess ?? []).map((a) => {
                const addon = addonsByID.get(a.addon_id)
                return (
                  <tr key={a.addon_id}>
                    <td>
                      {addon ? (
                        <Link to={`/addons/${addon.id}`}>{addon.name}</Link>
                      ) : (
                        <code>{a.addon_id}</code>
                      )}
                    </td>
                    <td>{addon?.visibility ?? "-"}</td>
                    <td>{new Date(a.created_at).toLocaleDateString()}</td>
                    <td>
                      <button
                        onClick={() => revokeAddon.mutate(a.addon_id)}
                        disabled={revokeAddon.isPending}
                      >
                        Revoke
                      </button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </section>
    </>
  )
}
