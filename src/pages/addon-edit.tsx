import { ArrowLeft, ArrowRight, FolderGit2 } from "lucide-react"
import { useState } from "react"
import type { FormEvent } from "react"
import { Link, useNavigate, useParams } from "react-router"
import {
  Button,
  buttonVariants,
  Card,
  Field,
  Input,
  Select,
  Spinner,
} from "@/components/ui"
import { useAddon } from "@/hooks/addons/use-addon"
import { useUpdateAddon } from "@/hooks/addons/use-update-addon"
import type { Addon, Visibility } from "@/lib/types"

export default function AddonEditPage() {
  const { id = "" } = useParams()
  const { data: addon, isLoading, isError } = useAddon(id)

  if (isLoading)
    return (
      <div className="flex justify-center py-16">
        <Spinner className="size-6" />
      </div>
    )
  if (isError || !addon)
    return <p className="text-danger">Could not load addon.</p>

  return <EditForm key={addon.id} addon={addon} />
}

function EditForm({ addon }: { addon: Addon }) {
  const update = useUpdateAddon(addon.id)
  const navigate = useNavigate()
  const [name, setName] = useState(addon.name)
  const [subpath, setSubpath] = useState(addon.subpath ?? "")
  const [visibility, setVisibility] = useState<Visibility>(addon.visibility)

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    update.mutate(
      { name: name.trim(), subpath, visibility },
      { onSuccess: () => navigate(`/addons/${addon.id}`) },
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <Link
        to={`/addons/${addon.id}`}
        className="inline-flex w-fit items-center gap-1 text-sm text-muted transition-colors hover:text-fg"
      >
        <ArrowLeft className="size-4" />
        Back to addon
      </Link>

      <div>
        <h1 className="text-2xl font-semibold">Edit {addon.name}</h1>
        <p className="mt-1 text-sm text-muted">
          Manage addon-level settings. Repository settings live on the repo.
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="p-5 lg:col-span-2">
          <h2 className="mb-4 font-medium">Addon settings</h2>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <Field
              label="Name"
              htmlFor="name"
              hint="The addon's unique name, e.g. vendor/name. Changing it updates how it is installed."
            >
              <Input
                id="name"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="vendor/name"
              />
            </Field>

            <Field
              label="Subpath"
              htmlFor="subpath"
              hint="Path to the module inside the repo. Leave empty if the manifest is at the repo root."
            >
              <Input
                id="subpath"
                value={subpath}
                onChange={(e) => setSubpath(e.target.value)}
                placeholder="addons/my_module"
              />
            </Field>

            <Field
              label="Visibility"
              htmlFor="visibility"
              hint="Public addons are visible to everyone. Private addons are restricted to you and granted groups."
            >
              <Select
                id="visibility"
                value={visibility}
                onChange={(e) => setVisibility(e.target.value as Visibility)}
              >
                <option value="public">Public</option>
                <option value="private">Private</option>
              </Select>
            </Field>

            {update.isError && (
              <p className="text-sm text-danger">
                Update failed: {update.error.message}
              </p>
            )}

            <div className="flex justify-end gap-2 border-t border-border pt-4">
              <Link
                to={`/addons/${addon.id}`}
                className={buttonVariants({ variant: "secondary" })}
              >
                Cancel
              </Link>
              <Button type="submit" loading={update.isPending}>
                Save changes
              </Button>
            </div>
          </form>
        </Card>

        <Card className="flex h-fit flex-col gap-4 p-5">
          <div className="flex items-center gap-2 font-medium">
            <FolderGit2 className="size-4 text-muted" />
            Repository
          </div>

          <dl className="flex flex-col gap-3 text-sm">
            <div className="flex flex-col gap-1">
              <dt className="text-xs font-medium uppercase tracking-wide text-muted">
                Git URL
              </dt>
              <dd>
                <code className="break-all">
                  {addon.repo?.git_url ?? "-"}
                </code>
              </dd>
            </div>
            <div className="flex flex-col gap-1">
              <dt className="text-xs font-medium uppercase tracking-wide text-muted">
                Default branch
              </dt>
              <dd>
                <code>{addon.repo?.default_branch ?? "-"}</code>
              </dd>
            </div>
            <div className="flex flex-col gap-1">
              <dt className="text-xs font-medium uppercase tracking-wide text-muted">
                Integration
              </dt>
              <dd>
                {addon.repo?.integration ? (
                  <span>
                    {addon.repo.integration.provider}
                    {addon.repo.integration.account_name
                      ? ` (${addon.repo.integration.account_name})`
                      : ""}
                  </span>
                ) : (
                  <span className="text-muted">none (anonymous clone)</span>
                )}
              </dd>
            </div>
          </dl>

          <Link
            to={`/repos/${addon.repo_id}`}
            className={buttonVariants({ variant: "secondary" })}
          >
            Manage repository
            <ArrowRight className="size-4" />
          </Link>
        </Card>
      </div>
    </div>
  )
}
