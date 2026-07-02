import { useQuery } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type { Repo } from "@/lib/types"

export function useRepo(id: string) {
  const api = useApiClient()
  return useQuery<Repo>({
    queryKey: queryKeys.repo(id),
    queryFn: () => api<Repo>(`/api/v1/repos/${id}`),
    enabled: !!id,
  })
}
