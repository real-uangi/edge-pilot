import { lazy, Suspense, useEffect } from "react";
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
import { FullscreenState } from "./components/FullscreenState";

const LoginPage = lazy(async () => {
  const module = await import("./pages/LoginPage");
  return { default: module.LoginPage };
});

const DashboardPage = lazy(async () => {
  const module = await import("./pages/DashboardPage");
  return { default: module.DashboardPage };
});

const ServicesPage = lazy(async () => {
  const module = await import("./pages/ServicesPage");
  return { default: module.ServicesPage };
});

const ServiceEditorPage = lazy(async () => {
  const module = await import("./pages/ServiceEditorPage");
  return { default: module.ServiceEditorPage };
});

const AgentsPage = lazy(async () => {
  const module = await import("./pages/AgentsPage");
  return { default: module.AgentsPage };
});

const RegistryCredentialsPage = lazy(async () => {
  const module = await import("./pages/RegistryCredentialsPage");
  return { default: module.RegistryCredentialsPage };
});

const AgentDetailPage = lazy(async () => {
  const module = await import("./pages/AgentDetailPage");
  return { default: module.AgentDetailPage };
});

const ReleasesPage = lazy(async () => {
  const module = await import("./pages/ReleasesPage");
  return { default: module.ReleasesPage };
});

const ReleaseDetailPage = lazy(async () => {
  const module = await import("./pages/ReleaseDetailPage");
  return { default: module.ReleaseDetailPage };
});

function RouteSuspense({
  children,
  title,
}: {
  children: React.ReactNode;
  title: string;
}) {
  return <Suspense fallback={<FullscreenState title={title} />}>{children}</Suspense>;
}

function LoginRoute() {
  const sessionQuery = useQuery({
    queryKey: ["session"],
    queryFn: api.me,
  });

  if (sessionQuery.isPending) {
    return <FullscreenState title="正在检查登录态" />;
  }
  if (sessionQuery.isSuccess) {
    return <Navigate to="/" replace />;
  }
  return (
    <RouteSuspense title="正在加载登录页">
      <LoginPage />
    </RouteSuspense>
  );
}

function ProtectedOutlet() {
  return (
    <RouteSuspense title="正在加载页面">
      <Outlet />
    </RouteSuspense>
  );
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
    return <FullscreenState title="正在进入管理面板" />;
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
      <ProtectedOutlet />
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
      { path: "registry-credentials", element: <RegistryCredentialsPage /> },
      { path: "agents", element: <AgentsPage /> },
      { path: "agents/:id", element: <AgentDetailPage /> },
      { path: "releases", element: <ReleasesPage /> },
      { path: "releases/:id", element: <ReleaseDetailPage /> },
    ],
  },
]);
