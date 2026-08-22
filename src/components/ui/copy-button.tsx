import { Check, Copy } from "lucide-react"
import { useState } from "react"
import { Button } from "./button"

export function CopyButton({
  value,
  size = "sm",
  className,
}: {
  value: string
  size?: "sm" | "icon"
  className?: string
}) {
  const [copied, setCopied] = useState(false)
  return (
    <Button
      type="button"
      variant="ghost"
      size={size}
      className={className}
      aria-label="Copy"
      disabled={!value}
      onClick={() => {
        if (!value) return
        navigator.clipboard?.writeText(value)
        setCopied(true)
        window.setTimeout(() => setCopied(false), 1500)
      }}
    >
      {copied ? (
        <Check className="size-4 text-success" />
      ) : (
        <Copy className="size-4" />
      )}
    </Button>
  )
}
