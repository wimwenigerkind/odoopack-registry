import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type { Repo, UpdateRepoRequest } from "@/lib/types"

export function useUpdateRepo(id: string) {
  const api = useApiClient()
  const qc = useQueryClient()

  return useMutation<Repo, Error, UpdateRepoRequest>({
    mutationFn: (req) =>
      api<Repo>(`/api/v1/repos/${id}`, {
        method: "PUT",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.repo(id) })
      qc.invalidateQueries({ queryKey: queryKeys.myRepos() })
    },
  })
}
