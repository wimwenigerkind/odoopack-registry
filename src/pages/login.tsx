import { Package } from "lucide-react"
import { Link, Navigate, useSearchParams } from "react-router"
import {
  buttonVariants,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Spinner,
} from "@/components/ui"
import { useMe } from "@/hooks/auth/use-me"
import { useProviders } from "@/hooks/auth/use-providers"
import { cn } from "@/lib/cn"

function providerLoginHref(name: string, returnTo: string) {
  return `/auth/${name}/login?return_to=${encodeURIComponent(returnTo)}`
}

export default function LoginPage() {
  const { data: user, isLoading: meLoading } = useMe()
  const { data, isLoading } = useProviders()
  const [params] = useSearchParams()

  const returnTo = params.get("return_to") || "/"
  const providers = data?.providers ?? []

  if (!meLoading && user) return <Navigate to={returnTo} replace />

  return (
    <div className="flex min-h-dvh flex-col items-center justify-center bg-canvas px-4">
      <div className="w-full max-w-sm">
        <Link
          to="/"
          className="mb-8 flex items-center justify-center gap-2 text-lg font-semibold"
        >
          <Package className="size-6 text-accent" />
          Odoopack
        </Link>

        <Card>
          <CardHeader>
            <CardTitle>Sign in</CardTitle>
            <CardDescription>
              Continue with one of the providers below.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            {isLoading ? (
              <div className="flex justify-center py-4">
                <Spinner />
              </div>
            ) : providers.length === 0 ? (
              <p className="text-sm text-muted">
                No login providers configured.
              </p>
            ) : (
              providers.map((p) => (
                <a
                  key={p.name}
                  href={providerLoginHref(p.name, returnTo)}
                  className={cn(buttonVariants({ variant: "secondary" }), "w-full")}
                >
                  Continue with <span className="capitalize">{p.name}</span>
                </a>
              ))
            )}
          </CardContent>
        </Card>

        <p className="mt-6 text-center text-sm text-muted">
          <Link to="/" className="hover:text-fg">
            Back to home
          </Link>
        </p>
      </div>
    </div>
  )
}
