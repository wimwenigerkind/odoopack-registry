import { cva, type VariantProps } from "class-variance-authority"
import type { HTMLAttributes } from "react"
import { cn } from "@/lib/cn"
import type { VersionStatus } from "@/lib/types"

const badgeVariants = cva(
  "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium capitalize",
  {
    variants: {
      variant: {
        neutral: "bg-fg/10 text-fg",
        accent: "bg-accent/10 text-accent",
        success: "bg-success/10 text-success",
        warning: "bg-warning/10 text-warning",
        danger: "bg-danger/10 text-danger",
        info: "bg-info/10 text-info",
      },
    },
    defaultVariants: { variant: "neutral" },
  },
)

type BadgeProps = HTMLAttributes<HTMLSpanElement> &
  VariantProps<typeof badgeVariants>

export function Badge({ className, variant, ...props }: BadgeProps) {
  return (
    <span className={cn(badgeVariants({ variant }), className)} {...props} />
  )
}

const statusVariant: Record<
  VersionStatus,
  VariantProps<typeof badgeVariants>["variant"]
> = {
  pending: "neutral",
  building: "info",
  ready: "success",
  failed: "danger",
}

export function StatusBadge({ status }: { status: VersionStatus }) {
  return <Badge variant={statusVariant[status]}>{status}</Badge>
}
