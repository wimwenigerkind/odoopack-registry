import { Package, Plus } from "lucide-react"
import { Link, useSearchParams } from "react-router"
import { Badge, buttonVariants, Card, EmptyState } from "@/components/ui"
import { useAddons } from "@/hooks/addons/use-addons"
import { useMe } from "@/hooks/auth/use-me"
import type { Addon } from "@/lib/types"

export default function HomePage() {
  const { data, isLoading } = useAddons()
  const { data: user } = useMe()
  const [params] = useSearchParams()

  const rawQuery = params.get("q")?.trim() ?? ""
  const query = rawQuery.toLowerCase()
  const all = data ?? []
  const addons = query
    ? all.filter((a) => a.name.toLowerCase().includes(query))
    : all

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between gap-4">
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

      {isLoading ? (
        <LoadingGrid />
      ) : addons.length === 0 ? (
        <EmptyState
          icon={Package}
          title={rawQuery ? `No addons match "${rawQuery}"` : "No addons yet"}
          description={
            rawQuery
              ? "Try a different search term."
              : user
                ? "Register your first addon to get started."
                : "Nothing to browse yet."
          }
          action={
            !rawQuery && user ? (
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
        <div
          key={i}
          className="h-28 animate-pulse rounded-xl border border-border bg-surface"
        />
      ))}
    </div>
  )
}
