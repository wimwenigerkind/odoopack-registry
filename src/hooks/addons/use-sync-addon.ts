import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"

export function useSyncAddon(id: string) {
  const api = useApiClient()
  const qc = useQueryClient()

  return useMutation({
    mutationFn: () =>
      api<void>(`/api/v1/addons/${id}/sync`, { method: "POST" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.addon(id) })
      qc.invalidateQueries({ queryKey: queryKeys.addons() })
    },
  })
}
