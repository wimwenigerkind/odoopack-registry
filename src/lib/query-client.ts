import { MutationCache, QueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

declare module "@tanstack/react-query" {
  interface Register {
    mutationMeta: { successMessage?: string }
  }
}

export const queryClient = new QueryClient({
  mutationCache: new MutationCache({
    onError: (error) =>
      toast.error(
        error instanceof Error ? error.message : "Something went wrong",
      ),
    onSuccess: (_data, _variables, _context, mutation) => {
      const message = mutation.meta?.successMessage
      if (message) toast.success(message)
    },
  }),
})
