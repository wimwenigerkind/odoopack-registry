import { useMutation } from "@tanstack/react-query"
import { useApiClient } from "@/lib/api"

interface CreateWorkspaceHookResponse {
  hook_uuid: string
  workspace: string
}

export function useCreateWorkspaceHook(integrationID: string) {
  const api = useApiClient()
  return useMutation<CreateWorkspaceHookResponse, Error, string>({
    meta: { successMessage: "Workspace hook created" },
    mutationFn: (workspace) =>
      api<CreateWorkspaceHookResponse>(
        `/api/v1/me/integrations/${integrationID}/workspace-hooks`,
        {
          method: "POST",
          body: JSON.stringify({ workspace }),
        },
      ),
  })
}
