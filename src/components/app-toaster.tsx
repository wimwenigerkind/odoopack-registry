import { useEffect, useState } from "react"
import { Toaster } from "sonner"

export function AppToaster() {
  const [dark, setDark] = useState(() =>
    document.documentElement.classList.contains("dark"),
  )

  useEffect(() => {
    const el = document.documentElement
    const observer = new MutationObserver(() =>
      setDark(el.classList.contains("dark")),
    )
    observer.observe(el, { attributes: true, attributeFilter: ["class"] })
    return () => observer.disconnect()
  }, [])

  return (
    <Toaster
      theme={dark ? "dark" : "light"}
      position="top-right"
      richColors
      closeButton
    />
  )
}
