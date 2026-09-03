import {useQuery} from "@tanstack/react-query"
import {useApiClient} from "@/lib/api"
import {queryKeys} from "@/lib/query-keys"
import type {AddonListResponse} from "@/lib/types.ts";

export function useAddons(
  params: { q?: string; series?: string; page?: number; per_page?: number } = {},
) {
  const api = useApiClient()
  const search = new URLSearchParams()
  if (params.q) search.set("q", params.q)
  if (params.series) search.set("series", params.series)
  if (params.page && params.page > 1) search.set("page", String(params.page))
  if (params.per_page) search.set("per_page", String(params.per_page))
  const qs = search.toString()

  return useQuery<AddonListResponse>({
    queryKey: queryKeys.addons(qs),
    queryFn: () => api<AddonListResponse>(`/api/v1/addons${qs ? "?" + qs : ""}`),
  })
}
