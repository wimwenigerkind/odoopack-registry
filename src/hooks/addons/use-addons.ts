import {useInfiniteQuery, useQuery} from "@tanstack/react-query"
import {useApiClient} from "@/lib/api"
import {queryKeys} from "@/lib/query-keys"
import type {AddonPage} from "@/lib/types.ts";

export function useAddons(params: { q?: string; series?: string } = {}) {
  const api = useApiClient()
  return useInfiniteQuery({
    queryKey: queryKeys.addons(JSON.stringify(params)),
    initialPageParam: "",
    queryFn: ({ pageParam }) => {
      const search = new URLSearchParams()
      if (params.q) search.set("q", params.q)
      if (params.series) search.set("series", params.series)
      if (pageParam) search.set("cursor", pageParam as string)
      const qs = search.toString()
      return api<AddonPage>(`/api/v1/addons${qs ? "?" + qs : ""}`)
    },
    getNextPageParam: (last) => last.meta.next_cursor || undefined,
  })
}

export function useAddonOptions() {
  const api = useApiClient()
  return useQuery<AddonPage>({
    queryKey: queryKeys.addons("options"),
    queryFn: () => api<AddonPage>("/api/v1/addons?limit=100"),
  })
}
