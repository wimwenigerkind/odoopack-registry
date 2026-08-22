import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type { ApiToken, CreateTokenRequest } from "@/lib/types"

export function useTokens() {
  const api = useApiClient()
  return useQuery<ApiToken[]>({
    queryKey: queryKeys.tokens(),
    queryFn: () => api<ApiToken[]>("/api/v1/me/tokens"),
  })
}

export function useCreateToken() {
  const api = useApiClient()
  const qc = useQueryClient()
  return useMutation<ApiToken, Error, CreateTokenRequest>({
    meta: { successMessage: "Token created" },
    mutationFn: (req) =>
      api<ApiToken>("/api/v1/me/tokens", {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.tokens() })
    },
  })
}

export function useDeleteToken() {
  const api = useApiClient()
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    meta: { successMessage: "Token revoked" },
    mutationFn: (id) =>
      api<void>(`/api/v1/me/tokens/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.tokens() })
    },
  })
}
