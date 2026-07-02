import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"

export function useDeleteRepo() {
  const api = useApiClient()
  const qc = useQueryClient()

  return useMutation<void, Error, string>({
    mutationFn: (id) =>
      api<void>(`/api/v1/repos/${id}`, {
        method: "DELETE",
      }),
    onSuccess: (_, id) => {
      qc.invalidateQueries({ queryKey: queryKeys.repo(id) })
      qc.invalidateQueries({ queryKey: queryKeys.myRepos() })
    },
  })
}
