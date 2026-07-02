import { useQuery } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type { Repo } from "@/lib/types"

export function useMyRepos() {
  const api = useApiClient()
  return useQuery<Repo[]>({
    queryKey: queryKeys.myRepos(),
    queryFn: () => api<Repo[]>("/api/v1/me/repos"),
  })
}
