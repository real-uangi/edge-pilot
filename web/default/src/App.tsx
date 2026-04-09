import { useEffect } from "react";
import {
  Navigate,
  Outlet,
  createBrowserRouter,
  useLocation,
  useNavigate,
} from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./lib/api";
import { AppShell } from "./components/AppShell";
import { LoginPage } from "./pages/LoginPage";
import { DashboardPage } from "./pages/DashboardPage";
import { ServicesPage } from "./pages/ServicesPage";
import { ServiceEditorPage } from "./pages/ServiceEditorPage";
import { AgentsPage } from "./pages/AgentsPage";
import { AgentDetailPage } from "./pages/AgentDetailPage";
import { ReleasesPage } from "./pages/ReleasesPage";
import { ReleaseDetailPage } from "./pages/ReleaseDetailPage";

function FullscreenState({ title }: { title: string }) {
  return (
    <div
      style={{
        minHeight: "100vh",
        display: "grid",
        placeItems: "center",
        background: "#fff",
        color: "#000",
        fontSize: "18px",
        fontWeight: 600,
      }}
    >
      {title}
    </div>
  );
}

function LoginRoute() {
  const sessionQuery = useQuery({
    queryKey: ["session"],
    queryFn: api.me,
  });

  if (sessionQuery.isPending) {
    return <FullscreenState title="Checking session" />;
  }
  if (sessionQuery.isSuccess) {
    return <Navigate to="/" replace />;
  }
  return <LoginPage />;
}

function ProtectedLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const sessionQuery = useQuery({
    queryKey: ["session"],
    queryFn: api.me,
  });

  const logoutMutation = useMutation({
    mutationFn: api.logout,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["session"] });
      navigate("/login", { replace: true });
    },
  });

  useEffect(() => {
    if (sessionQuery.isError) {
      navigate("/login", {
        replace: true,
        state: { from: location.pathname },
      });
    }
  }, [location.pathname, navigate, sessionQuery.isError]);

  if (sessionQuery.isPending) {
    return <FullscreenState title="Loading control plane" />;
  }
  if (!sessionQuery.data) {
    return null;
  }

  return (
    <AppShell
      username={sessionQuery.data.username}
      loggingOut={logoutMutation.isPending}
      onLogout={() => logoutMutation.mutate()}
    >
      <Outlet />
    </AppShell>
  );
}

export const router = createBrowserRouter([
  {
    path: "/login",
    element: <LoginRoute />,
  },
  {
    path: "/",
    element: <ProtectedLayout />,
    children: [
      { index: true, element: <DashboardPage /> },
      { path: "services", element: <ServicesPage /> },
      { path: "services/new", element: <ServiceEditorPage /> },
      { path: "services/:id", element: <ServiceEditorPage /> },
      { path: "agents", element: <AgentsPage /> },
      { path: "agents/:id", element: <AgentDetailPage /> },
      { path: "releases", element: <ReleasesPage /> },
      { path: "releases/:id", element: <ReleaseDetailPage /> },
    ],
  },
]);
