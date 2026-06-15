import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type { Addon, RegisterAddonRequest } from "@/lib/types"

export function useRegisterAddon() {
  const api = useApiClient()
  const qc = useQueryClient()

  return useMutation<Addon, Error, RegisterAddonRequest>({
    mutationFn: (req) =>
      api<Addon>("/api/v1/addons", {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.addons() })
    },
  })
}
