import { useEffect, useState } from "react"

interface AvatarProps {
  email: string
  size?: number
  alt?: string
}

export function Avatar({ email, size = 32, alt }: AvatarProps) {
  const [hash, setHash] = useState<string>("")

  useEffect(() => {
    let cancelled = false
    hashEmail(email).then((h) => {
      if (!cancelled) setHash(h)
    })
    return () => {
      cancelled = true
    }
  }, [email])

  if (!hash) return null

  return (
    <img
      src={`https://www.gravatar.com/avatar/${hash}?s=${size * 2}&d=identicon`}
      width={size}
      height={size}
      alt={alt ?? email}
    />
  )
}

async function hashEmail(email: string): Promise<string> {
  const normalized = email.trim().toLowerCase()
  const data = new TextEncoder().encode(normalized)
  const buf = await crypto.subtle.digest("SHA-256", data)
  return Array.from(new Uint8Array(buf))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("")
}
