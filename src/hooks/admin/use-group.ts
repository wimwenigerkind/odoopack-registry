import { useQuery } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type { Group } from "@/lib/types"

export function useGroup(id: string) {
  const api = useApiClient()
  return useQuery<Group>({
    queryKey: queryKeys.group(id),
    queryFn: () => api<Group>(`/api/v1/groups/${id}`),
    enabled: !!id,
  })
}
