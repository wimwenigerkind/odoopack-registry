import { ArrowLeft } from "lucide-react"
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
  const [subpath, setSubpath] = useState(addon.subpath ?? "")
  const [visibility, setVisibility] = useState<Visibility>(addon.visibility)

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()
    update.mutate(
      { subpath, visibility },
      { onSuccess: () => navigate(`/addons/${addon.id}`) },
    )
  }

  return (
    <div className="mx-auto flex max-w-xl flex-col gap-6">
      <Link
        to={`/addons/${addon.id}`}
        className="inline-flex w-fit items-center gap-1 text-sm text-muted transition-colors hover:text-fg"
      >
        <ArrowLeft className="size-4" />
        Back
      </Link>

      <div>
        <h1 className="text-2xl font-semibold">Edit {addon.name}</h1>
        <p className="mt-1 text-sm text-muted">
          Repo-level settings (git URL, default branch, integration) live on the{" "}
          <Link to={`/repos/${addon.repo_id}`} className="text-accent">
            repo
          </Link>
          .
        </p>
      </div>

      <Card className="p-5">
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <Field
            label="Subpath"
            htmlFor="subpath"
            hint="Leave empty if the manifest is at the repo root."
          >
            <Input
              id="subpath"
              value={subpath}
              onChange={(e) => setSubpath(e.target.value)}
              placeholder="addons/my_module"
            />
          </Field>

          <Field label="Visibility" htmlFor="visibility">
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

          <div className="flex justify-end gap-2">
            <Link
              to={`/addons/${addon.id}`}
              className={buttonVariants({ variant: "secondary" })}
            >
              Cancel
            </Link>
            <Button type="submit" loading={update.isPending}>
              Save
            </Button>
          </div>
        </form>
      </Card>
    </div>
  )
}
