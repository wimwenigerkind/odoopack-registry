import { useQuery } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type { Addon } from "@/lib/types"

export function useAddon(id: string) {
  const api = useApiClient()
  return useQuery<Addon>({
    queryKey: queryKeys.addon(id),
    queryFn: () => api<Addon>(`/api/v1/addons/${id}`),
    enabled: !!id,
    refetchInterval: (query) => {
      const addon = query.state.data
      if (!addon?.versions?.length) return false
      const hasInProgress = addon.versions.some(
        (v) => v.status === "pending" || v.status === "building",
      )
      return hasInProgress ? 2000 : false
    },
  })
}
