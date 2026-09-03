import { Eye, EyeOff } from "lucide-react"
import { useState } from "react"
import { Button } from "./button"
import { CopyButton } from "./copy-button"

const MASK = "•".repeat(12)

export function SecretValue({
  value,
  emptyText = "-",
  className,
}: {
  value: string
  emptyText?: string
  className?: string
}) {
  const [revealed, setRevealed] = useState(false)
  const hasValue = value !== ""

  return (
    <div className={`flex items-center gap-1 ${className ?? ""}`}>
      <code className="flex-1 truncate rounded-md bg-fg/5 px-2 py-1 text-xs">
        {hasValue ? (revealed ? value : MASK) : emptyText}
      </code>
      {hasValue && (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          aria-label={revealed ? "Hide secret" : "Reveal secret"}
          onClick={() => setRevealed((r) => !r)}
        >
          {revealed ? (
            <EyeOff className="size-4" />
          ) : (
            <Eye className="size-4" />
          )}
        </Button>
      )}
      <CopyButton value={value} />
    </div>
  )
}
