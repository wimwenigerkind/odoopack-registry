import { useQuery } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { ApiError } from "@/lib/api-error"
import { queryKeys } from "@/lib/query-keys"
import type { ReadmeResponse } from "@/lib/types"

export function useAddonReadme(id: string, version: string) {
  const api = useApiClient()
  return useQuery<ReadmeResponse | null>({
    queryKey: queryKeys.addonReadme(id, version),
    queryFn: async () => {
      try {
        return await api<ReadmeResponse>(
          `/api/v1/addons/${id}/versions/${encodeURIComponent(version)}/readme`,
        )
      } catch (err) {
        if (err instanceof ApiError && err.status === 404) return null
        throw err
      }
    },
    enabled: !!id && !!version,
  })
}
