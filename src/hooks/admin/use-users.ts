import { useQuery } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type { User } from "@/lib/types"

export function useUsers() {
  const api = useApiClient()
  return useQuery<User[]>({
    queryKey: queryKeys.users(),
    queryFn: () => api<User[]>("/api/v1/users"),
  })
}
