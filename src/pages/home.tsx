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
import { PageHeader } from "@/components/page-header"
import { useAddons } from "@/hooks/addons/use-addons"
import { useSeries } from "@/hooks/addons/use-series"
import { useMe } from "@/hooks/auth/use-me"
import type { Addon } from "@/lib/types"

export default function HomePage() {
  const { data: user } = useMe()
  const { data: seriesOptions } = useSeries()
  const [params, setParams] = useSearchParams()

  const q = params.get("q")?.trim() ?? ""
  const series = params.get("series") ?? ""

  const seriesFilterOptions =
    series && !(seriesOptions ?? []).includes(series)
      ? [series, ...(seriesOptions ?? [])]
      : (seriesOptions ?? [])

  const {
    data,
    isLoading,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useAddons({ q, series })
  const addons = data?.pages.flatMap((p) => p.data) ?? []

  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    setParams(next)
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Addons"
        description="Browse and install Odoo addons."
        breadcrumbs={[{ label: "Addons" }]}
        actions={
          user ? (
            <Link to="/addons/new" className={buttonVariants()}>
              <Plus className="size-4" />
              Register addon
            </Link>
          ) : undefined
        }
      />

      <div className="flex flex-wrap items-center justify-between gap-3">
        <Select
          value={series}
          onChange={(e) => setParam("series", e.target.value)}
          className="h-9 w-auto"
        >
          <option value="">All series</option>
          {seriesFilterOptions.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </Select>
        {!isLoading && addons.length > 0 && (
          <span className="text-sm text-muted-foreground">
            {addons.length}
            {hasNextPage ? "+" : ""} addon{addons.length === 1 ? "" : "s"}
          </span>
        )}
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
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5">
            {addons.map((addon) => (
              <AddonCard key={addon.id} addon={addon} />
            ))}
          </div>
          {hasNextPage && (
            <div className="flex justify-center">
              <Button
                variant="secondary"
                loading={isFetchingNextPage}
                onClick={() => fetchNextPage()}
              >
                Load more
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}

function AddonCard({ addon }: { addon: Addon }) {
  const versions = addon.versions ?? []
  const count = versions.length
  const ready = versions.filter((v) => v.status === "ready")
  const latest = versions.find((v) => v.is_latest) ?? ready[0] ?? versions[0]
  const seriesList = Array.from(
    new Set(ready.map((v) => v.series).filter((s): s is string => Boolean(s))),
  ).slice(0, 4)

  return (
    <Link to={`/addons/${addon.id}`} className="group block h-full">
      <Card className="flex h-full flex-col gap-4 p-5 transition-all hover:border-primary/50 hover:shadow-sm">
        <div className="flex items-start justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Package className="size-5" />
            </span>
            <div className="min-w-0">
              <div className="truncate font-medium group-hover:text-primary">
                {addon.name}
              </div>
              {latest?.version && (
                <div className="truncate text-xs text-muted-foreground">
                  v{latest.version}
                </div>
              )}
            </div>
          </div>
          <Badge variant={addon.visibility === "public" ? "neutral" : "warning"}>
            {addon.visibility}
          </Badge>
        </div>

        {latest?.summary && (
          <p className="line-clamp-2 text-sm text-muted-foreground">
            {latest.summary}
          </p>
        )}

        {seriesList.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {seriesList.map((s) => (
              <Badge key={s} variant="accent">
                {s}
              </Badge>
            ))}
          </div>
        )}

        <div className="mt-auto flex items-center justify-between border-t border-border pt-3 text-xs text-muted-foreground">
          <span>
            {count} version{count === 1 ? "" : "s"}
          </span>
          <span>Updated {new Date(addon.updated_at).toLocaleDateString()}</span>
        </div>
      </Card>
    </Link>
  )
}

function LoadingGrid() {
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5">
      {Array.from({ length: 10 }).map((_, i) => (
        <Skeleton key={i} className="h-44 rounded-xl" />
      ))}
    </div>
  )
}
