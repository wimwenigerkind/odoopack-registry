import HomePage from "@/pages/home.tsx";
import {createBrowserRouter, RouterProvider} from "react-router";
import RootLayout from "@/layouts/root.tsx";
import AddonDetailPage from "@/pages/addon-detail.tsx";
import AddonEditPage from "@/pages/addon-edit.tsx";
import AddonNewPage from "@/pages/addon-new.tsx";
import RepoDetailPage from "@/pages/repo-detail.tsx";
import LoginPage from "@/pages/login.tsx";
import ProfilePage from "@/pages/profile.tsx";
import AdminUsersPage from "@/pages/admin/users.tsx";
import AdminGroupsPage from "@/pages/admin/groups.tsx";
import AdminGroupDetailPage from "@/pages/admin/group-detail.tsx";
import { RequireAdmin } from "@/components/require-admin";

const router = createBrowserRouter([
  { path: "/login", element: <LoginPage/> },
  {
    element: <RootLayout/>,
    children: [
      {path:"/", element: <HomePage/>},
      { path: "/profile", element: <ProfilePage/> },
      { path: "/addons/new", element: <AddonNewPage/> },
      { path: "/addons/:id", element: <AddonDetailPage/> },
      { path: "/addons/:id/edit", element: <AddonEditPage/> },
      { path: "/repos/:id", element: <RepoDetailPage/> },
    ]
  },
  {
    element: <RequireAdmin><RootLayout/></RequireAdmin>,
    children: [
      { path: "/admin/users", element: <AdminUsersPage/> },
      { path: "/admin/groups", element: <AdminGroupsPage/> },
      { path: "/admin/groups/:id", element: <AdminGroupDetailPage/> },
    ]
  }
]);

function App() {
  return (
    <RouterProvider router={router}/>
  )
}

export default App
