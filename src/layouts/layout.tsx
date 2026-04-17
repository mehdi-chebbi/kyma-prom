import { Outlet } from "react-router-dom";
import { Header } from "../components/header/header";
import "./layout.css";
import { Sidebar } from "../components/sidebar/sidebar";
import { ToolsDock } from "../components/tools-dock/tools-dock";
import { ActiveUsersSidebarContent } from "../components/sidebar/active-users-sidebar-content/active-users-sidebar-content";

export default function Layout() {
  return (
    <div className="layout">
      <Header/>

      <ToolsDock />
      <Sidebar side="right">
        <ActiveUsersSidebarContent/>
      </Sidebar>

      <main className="layout-content">
        <Outlet />
      </main>
    </div>
  );
}
