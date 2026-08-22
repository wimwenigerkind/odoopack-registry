import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"
import { queryKeys } from "@/lib/query-keys"
import type {
  IntegrationProvidersResponse,
  OAuthIntegration,
} from "@/lib/types"

export function useIntegrations() {
  const api = useApiClient()
  return useQuery<OAuthIntegration[]>({
    queryKey: queryKeys.integrations(),
    queryFn: () => api<OAuthIntegration[]>("/api/v1/me/integrations"),
  })
}

export function useIntegrationProviders() {
  const api = useApiClient()
  return useQuery<IntegrationProvidersResponse>({
    queryKey: queryKeys.integrationProviders(),
    queryFn: () =>
      api<IntegrationProvidersResponse>("/integrations/providers"),
  })
}

export function useDeleteIntegration() {
  const api = useApiClient()
  const qc = useQueryClient()
  return useMutation<void, Error, string>({
    meta: { successMessage: "Integration disconnected" },
    mutationFn: (id) =>
      api<void>(`/api/v1/me/integrations/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: queryKeys.integrations() })
    },
  })
}
