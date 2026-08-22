import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"

export function useDeleteAddon() {
  const api = useApiClient()
  const qc = useQueryClient()

  return useMutation<void, Error, string>({
    meta: { successMessage: "Addon deleted" },
    mutationFn: (id) =>
      api<void>(`/api/v1/addons/${id}`, {
        method: "DELETE",
      }),
    onSuccess: (_, id) => {
      qc.invalidateQueries({ queryKey: queryKeys.addon(id) })
      qc.invalidateQueries({ queryKey: queryKeys.addons() })
      qc.invalidateQueries({ queryKey: queryKeys.myRepos() })
    },
  })
}
