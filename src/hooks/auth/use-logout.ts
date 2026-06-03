import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"

export function useLogout() {
  const api = useApiClient()
  const qc = useQueryClient()

  return useMutation({
    mutationFn: () => api<void>("/auth/logout", { method: "POST" }),
    onSuccess: () => {
      qc.setQueryData(queryKeys.me(), null)
      qc.invalidateQueries({ queryKey: queryKeys.me() })
    },
  })
}
