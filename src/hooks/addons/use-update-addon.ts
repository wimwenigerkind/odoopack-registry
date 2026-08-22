import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type { Addon, UpdateAddonRequest } from "@/lib/types"

export function useUpdateAddon(id: string) {
  const api = useApiClient()
  const qc = useQueryClient()

  return useMutation<Addon, Error, UpdateAddonRequest>({
    meta: { successMessage: "Changes saved" },
    mutationFn: (req) =>
      api<Addon>(`/api/v1/addons/${id}`, {
        method: "PUT",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.addon(id) })
      qc.invalidateQueries({ queryKey: queryKeys.addons() })
    },
  })
}
