import { lazy, Suspense, useEffect } from "react";
import {
  Navigate,
  Outlet,
  createBrowserRouter,
  useLocation,
  useNavigate,
} from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AppShell } from "../shared/components/AppShell";
import { FullscreenState } from "../shared/components/FullscreenState";
import { authApi } from "../features/auth/api";

const LoginPage = lazy(async () => {
  const module = await import("../features/auth/components/LoginPage");
  return { default: module.LoginPage };
});

const DashboardPage = lazy(async () => {
  const module = await import("../features/dashboard/components/DashboardPage");
  return { default: module.DashboardPage };
});

const SystemPerformancePage = lazy(async () => {
  const module = await import("../features/system-performance/components/SystemPerformancePage");
  return { default: module.SystemPerformancePage };
});

const ServicesPage = lazy(async () => {
  const module = await import("../features/services/components/ServicesPage");
  return { default: module.ServicesPage };
});

const ServiceEditorPage = lazy(async () => {
  const module = await import("../features/services/components/ServiceEditorPage");
  return { default: module.ServiceEditorPage };
});

const AgentsPage = lazy(async () => {
  const module = await import("../features/agents/components/AgentsPage");
  return { default: module.AgentsPage };
});

const RegistryCredentialsPage = lazy(async () => {
  const module = await import("../features/registry-credentials/components/RegistryCredentialsPage");
  return { default: module.RegistryCredentialsPage };
});

const AgentDetailPage = lazy(async () => {
  const module = await import("../features/agents/components/AgentDetailPage");
  return { default: module.AgentDetailPage };
});

const ReleasesPage = lazy(async () => {
  const module = await import("../features/releases/components/ReleasesPage");
  return { default: module.ReleasesPage };
});

const ReleaseDetailPage = lazy(async () => {
  const module = await import("../features/releases/components/ReleaseDetailPage");
  return { default: module.ReleaseDetailPage };
});

const JobsPage = lazy(async () => {
  const module = await import("../features/scheduler/components/JobsPage");
  return { default: module.JobsPage };
});

const RunsPage = lazy(async () => {
  const module = await import("../features/scheduler/components/RunsPage");
  return { default: module.RunsPage };
});

const ExecutorsPage = lazy(async () => {
  const module = await import("../features/scheduler/components/ExecutorsPage");
  return { default: module.ExecutorsPage };
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
    queryFn: authApi.me,
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
    queryFn: authApi.me,
  });

  const logoutMutation = useMutation({
    mutationFn: authApi.logout,
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
      { path: "system-performance", element: <SystemPerformancePage /> },
      { path: "services", element: <ServicesPage /> },
      { path: "services/new", element: <ServiceEditorPage /> },
      { path: "services/:id", element: <ServiceEditorPage /> },
      { path: "registry-credentials", element: <RegistryCredentialsPage /> },
      { path: "agents", element: <AgentsPage /> },
      { path: "agents/:id", element: <AgentDetailPage /> },
      { path: "releases", element: <ReleasesPage /> },
      { path: "releases/:id", element: <ReleaseDetailPage /> },
      { path: "scheduler", element: <JobsPage /> },
      { path: "scheduler/history", element: <RunsPage /> },
      { path: "scheduler/executors", element: <ExecutorsPage /> },
    ],
  },
]);
