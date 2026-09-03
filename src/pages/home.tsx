import { Package, Plus } from "lucide-react"
import { Link, useSearchParams } from "react-router"
import {
  Badge,
  Button,
  buttonVariants,
  Card,
  EmptyState,
  Select,
  Skeleton,
} from "@/components/ui"
import { useAddons } from "@/hooks/addons/use-addons"
import { useMe } from "@/hooks/auth/use-me"
import type { Addon } from "@/lib/types"

const SERIES = ["19.0", "18.0", "17.0", "16.0"]

export default function HomePage() {
  const { data: user } = useMe()
  const [params, setParams] = useSearchParams()

  const q = params.get("q")?.trim() ?? ""
  const series = params.get("series") ?? ""
  const page = Math.max(1, Number(params.get("page") ?? "1") || 1)

  const { data, isLoading } = useAddons({ q, series, page })
  const addons = data?.items ?? []
  const total = data?.total ?? 0
  const perPage = data?.per_page ?? 20
  const totalPages = Math.max(1, Math.ceil(total / perPage))

  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    if (key !== "page") next.delete("page")
    setParams(next)
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold">Addons</h1>
          <p className="text-sm text-muted">Browse and install Odoo addons.</p>
        </div>
        {user && (
          <Link to="/addons/new" className={buttonVariants()}>
            <Plus className="size-4" />
            Register addon
          </Link>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Select
          value={series}
          onChange={(e) => setParam("series", e.target.value)}
          className="h-9 w-auto"
        >
          <option value="">All series</option>
          {SERIES.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </Select>
        <span className="text-sm text-muted">
          {total} addon{total === 1 ? "" : "s"}
          {q ? ` matching "${q}"` : ""}
        </span>
      </div>

      {isLoading ? (
        <LoadingGrid />
      ) : addons.length === 0 ? (
        <EmptyState
          icon={Package}
          title={q ? `No addons match "${q}"` : "No addons yet"}
          description={
            q || series
              ? "Try a different search term or filter."
              : user
                ? "Register your first addon to get started."
                : "Nothing to browse yet."
          }
          action={
            !q && !series && user ? (
              <Link to="/addons/new" className={buttonVariants()}>
                <Plus className="size-4" />
                Register addon
              </Link>
            ) : undefined
          }
        />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {addons.map((addon) => (
            <AddonCard key={addon.id} addon={addon} />
          ))}
        </div>
      )}

      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-3">
          <Button
            variant="secondary"
            size="sm"
            disabled={page <= 1}
            onClick={() => setParam("page", String(page - 1))}
          >
            Previous
          </Button>
          <span className="text-sm text-muted">
            Page {page} of {totalPages}
          </span>
          <Button
            variant="secondary"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => setParam("page", String(page + 1))}
          >
            Next
          </Button>
        </div>
      )}
    </div>
  )
}

function AddonCard({ addon }: { addon: Addon }) {
  const versions = addon.versions?.length ?? 0
  return (
    <Link to={`/addons/${addon.id}`} className="group block h-full">
      <Card className="flex h-full flex-col gap-4 p-5 transition-colors hover:border-accent/50">
        <div className="flex items-start justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2">
            <Package className="size-4 shrink-0 text-muted" />
            <span className="truncate font-medium group-hover:text-accent">
              {addon.name}
            </span>
          </div>
          <Badge variant={addon.visibility === "public" ? "neutral" : "warning"}>
            {addon.visibility}
          </Badge>
        </div>
        <div className="mt-auto flex items-center justify-between text-xs text-muted">
          <span>
            {versions} version{versions === 1 ? "" : "s"}
          </span>
          <span>Updated {new Date(addon.updated_at).toLocaleDateString()}</span>
        </div>
      </Card>
    </Link>
  )
}

function LoadingGrid() {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <Skeleton key={i} className="h-28 rounded-xl" />
      ))}
    </div>
  )
}
