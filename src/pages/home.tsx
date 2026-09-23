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
import { useMe } from "@/hooks/auth/use-me"
import type { Addon } from "@/lib/types"

const SERIES = ["19.0", "18.0", "17.0", "16.0"]

export default function HomePage() {
  const { data: user } = useMe()
  const [params, setParams] = useSearchParams()

  const q = params.get("q")?.trim() ?? ""
  const series = params.get("series") ?? ""

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
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
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
  const versions = addon.versions?.length ?? 0
  return (
    <Link to={`/addons/${addon.id}`} className="group block h-full">
      <Card className="flex h-full flex-col gap-4 p-5 transition-colors hover:border-primary/50">
        <div className="flex items-start justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2">
            <Package className="size-4 shrink-0 text-muted-foreground" />
            <span className="truncate font-medium group-hover:text-primary">
              {addon.name}
            </span>
          </div>
          <Badge variant={addon.visibility === "public" ? "neutral" : "warning"}>
            {addon.visibility}
          </Badge>
        </div>
        <div className="mt-auto flex items-center justify-between text-xs text-muted-foreground">
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
