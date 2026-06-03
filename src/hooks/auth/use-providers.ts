import {useQuery} from "@tanstack/react-query"
import {useApiClient} from "@/lib/api"
import {queryKeys} from "@/lib/query-keys"
import type {ProvidersResponse} from "@/lib/types"

export function useProviders() {
  const api = useApiClient()

  return useQuery<ProvidersResponse>({
    queryKey: queryKeys.providers(),
    queryFn: () => api<ProvidersResponse>("/auth/providers"),
    staleTime: 5 * 60 * 1000,
  })
}
