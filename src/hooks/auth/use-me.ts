import {useQuery} from "@tanstack/react-query"
import {useApiClient} from "@/lib/api"
import {queryKeys} from "@/lib/query-keys"
import type {User} from "@/lib/types"
import {ApiError} from "@/lib/api-error"

export function useMe() {
  const api = useApiClient()

  return useQuery<User | null>({
    queryKey: queryKeys.me(),
    queryFn: async () => {
      try {
        return await api<User>("/api/v1/me")
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          return null
        }
        throw err
      }
    },
    staleTime: 60 * 1000,
  })
}
