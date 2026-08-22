import { Outlet } from "react-router"
import { TopBar } from "@/components/top-bar"

export default function RootLayout() {
  return (
    <div className="min-h-dvh">
      <TopBar />
      <main className="mx-auto max-w-[1100px] px-4 py-8">
        <Outlet />
      </main>
    </div>
  )
}
