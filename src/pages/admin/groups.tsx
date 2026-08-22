import { Plus, Trash2 } from "lucide-react"
import { useState } from "react"
import type { FormEvent } from "react"
import { Link } from "react-router"
import { AdminNav } from "@/components/admin-nav"
import {
  Button,
  Card,
  ConfirmDialog,
  EmptyState,
  Field,
  Input,
  Spinner,
  Table,
  TBody,
  TD,
  TH,
  THead,
  TR,
} from "@/components/ui"
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
    createGroup.mutate({ name: name.trim() }, { onSuccess: () => setName("") })
  }

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-semibold">Admin</h1>
      <AdminNav />

      <Card className="p-5">
        <form onSubmit={handleCreate} className="flex items-end gap-2">
          <div className="flex-1">
            <Field label="New group" htmlFor="group-name">
              <Input
                id="group-name"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Group name"
              />
            </Field>
          </div>
          <Button type="submit" loading={createGroup.isPending}>
            <Plus className="size-4" />
            Create
          </Button>
        </form>
        {createGroup.isError && (
          <p className="mt-2 text-sm text-danger">
            Failed to create: {createGroup.error.message}
          </p>
        )}
      </Card>

      {isLoading ? (
        <div className="flex justify-center py-8">
          <Spinner className="size-6" />
        </div>
      ) : isError ? (
        <p className="text-danger">Could not load groups.</p>
      ) : (groups ?? []).length === 0 ? (
        <EmptyState title="No groups yet" description="Create a group above." />
      ) : (
        <Table>
          <THead>
            <TR>
              <TH>Name</TH>
              <TH>Created</TH>
              <TH></TH>
            </TR>
          </THead>
          <TBody>
            {(groups ?? []).map((g) => (
              <TR key={g.id}>
                <TD className="font-medium">
                  <Link
                    to={`/admin/groups/${g.id}`}
                    className="hover:text-accent"
                  >
                    {g.name}
                  </Link>
                </TD>
                <TD className="text-muted">
                  {new Date(g.created_at).toLocaleDateString()}
                </TD>
                <TD className="text-right">
                  <ConfirmDialog
                    trigger={
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-danger hover:bg-danger/10 hover:text-danger"
                        aria-label={`Delete group ${g.name}`}
                      >
                        <Trash2 className="size-4" />
                      </Button>
                    }
                    title={`Delete group ${g.name}?`}
                    confirmLabel="Delete"
                    destructive
                    loading={deleteGroup.isPending}
                    onConfirm={() => deleteGroup.mutate(g.id)}
                  />
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}
    </div>
  )
}
