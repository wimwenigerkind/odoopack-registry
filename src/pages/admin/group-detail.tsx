import { ArrowLeft } from "lucide-react"
import { useMemo, useState } from "react"
import type { FormEvent } from "react"
import { Link, useParams } from "react-router"
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Select,
  Spinner,
  Table,
  TBody,
  TD,
  TH,
  THead,
  TR,
} from "@/components/ui"
import { useAddons } from "@/hooks/addons/use-addons"
import {
  useGrantGroupAddon,
  useGroupAddons,
  useRevokeGroupAddon,
} from "@/hooks/admin/use-group-addons"
import {
  useAddGroupMember,
  useGroupMembers,
  useRemoveGroupMember,
} from "@/hooks/admin/use-group-members"
import { useGroup } from "@/hooks/admin/use-group"
import { useUsers } from "@/hooks/admin/use-users"
import type { Addon, User } from "@/lib/types"

const dangerGhost = "text-danger hover:bg-danger/10 hover:text-danger"

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

  const usersByID = useMemo(
    () => new Map<string, User>((users ?? []).map((u) => [u.id, u])),
    [users],
  )
  const addonsByID = useMemo(
    () => new Map<string, Addon>((addons ?? []).map((a) => [a.id, a])),
    [addons],
  )

  if (isLoading)
    return (
      <div className="flex justify-center py-16">
        <Spinner className="size-6" />
      </div>
    )
  if (isError) return <p className="text-danger">Could not load group.</p>
  if (!group) return <p>Group not found.</p>

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
    <div className="flex flex-col gap-6">
      <Link
        to="/admin/groups"
        className="inline-flex w-fit items-center gap-1 text-sm text-muted transition-colors hover:text-fg"
      >
        <ArrowLeft className="size-4" />
        Back to groups
      </Link>

      <h1 className="text-2xl font-semibold">{group.name}</h1>

      <Card>
        <CardHeader>
          <CardTitle>Members ({members?.length ?? 0})</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <form onSubmit={handleAddMember} className="flex gap-2">
            <Select
              value={memberToAdd}
              onChange={(e) => setMemberToAdd(e.target.value)}
              className="flex-1"
            >
              <option value="">Select a user</option>
              {availableUsers.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.email} {u.username ? `(${u.username})` : ""}
                </option>
              ))}
            </Select>
            <Button
              type="submit"
              disabled={!memberToAdd}
              loading={addMember.isPending}
            >
              Add
            </Button>
          </form>

          {(members ?? []).length === 0 ? (
            <p className="text-sm text-muted">No members.</p>
          ) : (
            <Table>
              <THead>
                <TR>
                  <TH>User</TH>
                  <TH>Joined</TH>
                  <TH></TH>
                </TR>
              </THead>
              <TBody>
                {(members ?? []).map((m) => {
                  const u = usersByID.get(m.user_id)
                  return (
                    <TR key={m.user_id}>
                      <TD className="font-medium">
                        {u ? (
                          <>
                            {u.email}
                            {u.username && (
                              <span className="text-muted">
                                {" "}
                                ({u.username})
                              </span>
                            )}
                          </>
                        ) : (
                          <code className="text-xs">{m.user_id}</code>
                        )}
                      </TD>
                      <TD className="text-muted">
                        {new Date(m.created_at).toLocaleDateString()}
                      </TD>
                      <TD className="text-right">
                        <Button
                          variant="ghost"
                          size="sm"
                          className={dangerGhost}
                          disabled={removeMember.isPending}
                          onClick={() => removeMember.mutate(m.user_id)}
                        >
                          Remove
                        </Button>
                      </TD>
                    </TR>
                  )
                })}
              </TBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Addon access ({addonAccess?.length ?? 0})</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <form onSubmit={handleGrantAddon} className="flex gap-2">
            <Select
              value={addonToAdd}
              onChange={(e) => setAddonToAdd(e.target.value)}
              className="flex-1"
            >
              <option value="">Select an addon</option>
              {availableAddons.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name} ({a.visibility})
                </option>
              ))}
            </Select>
            <Button
              type="submit"
              disabled={!addonToAdd}
              loading={grantAddon.isPending}
            >
              Grant
            </Button>
          </form>

          {(addonAccess ?? []).length === 0 ? (
            <p className="text-sm text-muted">No addon access granted.</p>
          ) : (
            <Table>
              <THead>
                <TR>
                  <TH>Addon</TH>
                  <TH>Visibility</TH>
                  <TH>Granted</TH>
                  <TH></TH>
                </TR>
              </THead>
              <TBody>
                {(addonAccess ?? []).map((a) => {
                  const addon = addonsByID.get(a.addon_id)
                  return (
                    <TR key={a.addon_id}>
                      <TD className="font-medium">
                        {addon ? (
                          <Link
                            to={`/addons/${addon.id}`}
                            className="hover:text-accent"
                          >
                            {addon.name}
                          </Link>
                        ) : (
                          <code className="text-xs">{a.addon_id}</code>
                        )}
                      </TD>
                      <TD>
                        {addon ? (
                          <Badge
                            variant={
                              addon.visibility === "public"
                                ? "neutral"
                                : "warning"
                            }
                          >
                            {addon.visibility}
                          </Badge>
                        ) : (
                          "-"
                        )}
                      </TD>
                      <TD className="text-muted">
                        {new Date(a.created_at).toLocaleDateString()}
                      </TD>
                      <TD className="text-right">
                        <Button
                          variant="ghost"
                          size="sm"
                          className={dangerGhost}
                          disabled={revokeAddon.isPending}
                          onClick={() => revokeAddon.mutate(a.addon_id)}
                        >
                          Revoke
                        </Button>
                      </TD>
                    </TR>
                  )
                })}
              </TBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
