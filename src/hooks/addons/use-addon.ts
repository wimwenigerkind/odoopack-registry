import {useQuery} from "@tanstack/react-query"
import {useApiClient} from "@/lib/api"
import {queryKeys} from "@/lib/query-keys"
import type {Addon} from "@/lib/types.ts";

export function useAddon(id: string) {
  const api = useApiClient()
  return useQuery<Addon>({
    queryKey: queryKeys.addon(id),
    queryFn: () => api<Addon>(`/api/v1/addons/${id}`),
    enabled: !!id,
  })
}