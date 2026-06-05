import './App.css'
import HomePage from "@/pages/home.tsx";
import {createBrowserRouter, RouterProvider} from "react-router";
import RootLayout from "@/layouts/root.tsx";
import AddonDetailPage from "@/pages/addon-detail.tsx";
import AddonNewPage from "@/pages/addon-new.tsx";
import ProfilePage from "@/pages/profile.tsx";
import AdminUsersPage from "@/pages/admin/users.tsx";
import AdminGroupsPage from "@/pages/admin/groups.tsx";
import AdminGroupDetailPage from "@/pages/admin/group-detail.tsx";
import { RequireAdmin } from "@/components/require-admin";

const router = createBrowserRouter([
  {
    element: <RootLayout/>,
    children: [
      {path:"/", element: <HomePage/>},
      { path: "/profile", element: <ProfilePage/> },
      { path: "/addons/new", element: <AddonNewPage/> },
      { path: "/addons/:id", element: <AddonDetailPage/> },
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
