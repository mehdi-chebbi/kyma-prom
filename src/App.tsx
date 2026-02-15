import {
  BrowserRouter as Router,
  Route,
  Routes,
  Navigate,
  useNavigate,
  Outlet,
} from "react-router-dom";
import { useEffect } from "react";
import "./App.css";
import Login from "./pages/auth/login/login";
import Dashboard from "./pages/dashboard/dashboard";
import Layout from "./layouts/layout";
import Projects from "./pages/projects/projects";
import { UsersDashboard } from "./pages/users-dashboard/users-dashboard";
import { DepartmentsDashboard } from "./pages/departments-dashboard/departments-dashboard";
import { GroupsManagement } from "./pages/groups-management/groups-management";
import { ReposDashboard } from "./pages/repos-dashboard/repos-dashboard";
import Workspaces from "./pages/workspaces/workspaces";
import { checkExistingSession } from "./services/userService";

const ProtectedRoute = () => {
  const isAuthenticated = !!localStorage.getItem("token");
  return isAuthenticated ? <Outlet /> : <Navigate to="/login" replace />;
};

const PublicRoute = ({ children }: { children: React.ReactNode }) => {
  const isAuthenticated = !!localStorage.getItem("token");
  return isAuthenticated ? (
    <Navigate to="/dashboard" replace />
  ) : (
    <>{children}</>
  );
};

function SessionManager() {
  const navigate = useNavigate();

  useEffect(() => {
    checkExistingSession(navigate);
  }, [navigate]);

  return null;
}

function App() {
  return (
    <Router>
      <SessionManager />
      <Routes>
        <Route path="/" element={<Navigate to="/login" replace />} />
        <Route
          path="/login"
          element={
            <PublicRoute>
              <Login />
            </PublicRoute>
          }
        />

        <Route element={<ProtectedRoute />}>
          <Route element={<Layout />}>
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/projects" element={<Projects />} />
            <Route path="/workspaces" element={<Workspaces />} />
            <Route path="/management/users" element={<UsersDashboard />} />
            <Route
              path="/management/departments"
              element={<DepartmentsDashboard />}
            />
            <Route
              path="/management/groups"
              element={<GroupsManagement />}
            />
            <Route
              path="/management/repositories"
              element={<ReposDashboard />}
            />
          </Route>
        </Route>

        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </Router>
  );
}

export default App;
