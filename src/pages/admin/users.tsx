import { AdminNav } from "@/components/admin-nav"
import {
  Badge,
  Spinner,
  Table,
  TBody,
  TD,
  TH,
  THead,
  TR,
} from "@/components/ui"
import { useUsers } from "@/hooks/admin/use-users"

export default function AdminUsersPage() {
  const { data: users, isLoading, isError } = useUsers()

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-semibold">Admin</h1>
      <AdminNav />

      {isLoading ? (
        <div className="flex justify-center py-8">
          <Spinner className="size-6" />
        </div>
      ) : isError ? (
        <p className="text-danger">Could not load users.</p>
      ) : (
        <Table>
          <THead>
            <TR>
              <TH>Email</TH>
              <TH>Username</TH>
              <TH>Role</TH>
              <TH>ID</TH>
            </TR>
          </THead>
          <TBody>
            {(users ?? []).map((u) => (
              <TR key={u.id}>
                <TD className="font-medium">{u.email}</TD>
                <TD className="text-muted">{u.username || "-"}</TD>
                <TD>
                  {u.is_admin ? (
                    <Badge variant="accent">admin</Badge>
                  ) : (
                    <span className="text-muted">user</span>
                  )}
                </TD>
                <TD className="text-muted">
                  <code className="text-xs">{u.id}</code>
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}
    </div>
  )
}
