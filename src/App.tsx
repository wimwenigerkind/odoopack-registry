import './App.css'
import HomePage from "@/pages/home.tsx";
import {createBrowserRouter, RouterProvider} from "react-router";
import RootLayout from "@/layouts/root.tsx";
import AddonDetailPage from "@/pages/addon-detail.tsx";
import AddonNewPage from "@/pages/addon-new.tsx";
import ProfilePage from "@/pages/profile.tsx";

const router = createBrowserRouter([
  {
    element: <RootLayout/>,
    children: [
      {path:"/", element: <HomePage/>},
      { path: "/profile", element: <ProfilePage/> },
      { path: "/addons/new", element: <AddonNewPage/> },
      { path: "/addons/:id", element: <AddonDetailPage/> }
    ]
  }
]);

function App() {
  return (
    <RouterProvider router={router}/>
  )
}

export default App
