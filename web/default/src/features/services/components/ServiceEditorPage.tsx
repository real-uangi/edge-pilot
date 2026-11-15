import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Link,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { StatusPill } from "../../../shared/components/StatusPill";
import { ActionButton } from "../../../shared/components/ActionButton";
import { useDialog } from "../../../shared/components/DialogProvider";
import {
  EmptyState,
  ErrorState,
  LoadingState,
  QueryRefreshNotice,
} from "../../../shared/components/StateBlocks";
import { servicesApi } from "../api";
import type { ServiceFormValues } from "../../../shared/lib/forms";
import { toServicePayload } from "../../../shared/lib/forms";
import { ServiceForm } from "./ServiceForm";
import { ServiceSummary } from "./ServiceSummary";
import { ServiceObservability } from "./ServiceObservability";
import { ServiceReleaseCard } from "./ServiceReleaseCard";
import styles from "../../../styles/admin.module.css";

export function ServiceEditorPage() {
  const params = useParams();
  const navigate = useNavigate();
  const dialog = useDialog();
  const [searchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const [submitError, setSubmitError] = useState<string | null>(null);
  const serviceId = params.id;
  const isEdit = Boolean(serviceId);
  const activeTab =
    isEdit && searchParams.get("tab") !== "config" ? "overview" : "config";

  const serviceQuery = useQuery({
    queryKey: ["service", serviceId],
    queryFn: () => servicesApi.get(serviceId!),
    enabled: isEdit,
  });

  const saveMutation = useMutation({
    mutationFn: async (values: ServiceFormValues) => {
      const payload = toServicePayload(values);
      if (isEdit && serviceQuery.data) {
        payload.serviceKey = serviceQuery.data.serviceKey;
        payload.agentId = serviceQuery.data.agentId;
      }
      return isEdit
        ? servicesApi.update(serviceId!, payload)
        : servicesApi.create(payload);
    },
    onSuccess: async (service) => {
      queryClient.setQueryData(["service", service.id], service);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["services"] }),
        queryClient.invalidateQueries({ queryKey: ["overview"] }),
      ]);
      if (!isEdit) {
        navigate(`/services/${service.id}?tab=overview`, { replace: true });
      }
    },
    onError: (error) => setSubmitError(getErrorMessage(error)),
  });

  const deleteMutation = useMutation({
    mutationFn: (targetId: string) => servicesApi.delete(targetId),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["services"] }),
        queryClient.invalidateQueries({ queryKey: ["overview"] }),
      ]);
      navigate("/services", { replace: true });
    },
    onError: (error) => setSubmitError(getErrorMessage(error)),
  });

  if (isEdit && serviceQuery.isPending) {
    return (
      <div className={styles.page}>
        <LoadingState
          title="正在加载服务详情"
          message="正在拉取服务配置与运行摘要。"
        />
      </div>
    );
  }

  if (isEdit && serviceQuery.isError && !serviceQuery.data) {
    return (
      <div className={styles.page}>
        <ErrorState
          title="服务详情加载失败"
          message={getErrorMessage(serviceQuery.error)}
          onRetry={() => serviceQuery.refetch()}
        />
      </div>
    );
  }

  if (isEdit && !serviceQuery.data) {
    return (
      <div className={styles.page}>
        <EmptyState title="服务不存在" message="请返回服务列表后重新选择。" />
      </div>
    );
  }

  const handleDelete = async () => {
    if (!serviceQuery.data) {
      return;
    }
    const targetId = serviceQuery.data.id;
    const confirmed = await dialog.confirm({
      title: `删除服务 ${serviceQuery.data.name}`,
      message: `删除 ${serviceQuery.data.serviceKey}，并清理该服务的容器与代理配置。此操作无法撤销。`,
      confirmText: "删除服务",
      danger: true,
    });
    if (!confirmed) {
      return;
    }
    setSubmitError(null);
    deleteMutation.mutate(targetId);
  };

  return (
    <div className={styles.page}>
      <header className={styles.pageHeader}>
        <div className={styles.pageTitleGroup}>
          <h1 className={styles.sectionTitle}>
            {serviceQuery.data?.name ?? "新建服务"}
          </h1>
          {serviceQuery.data ? (
            <p className={`${styles.sectionCopy} ${styles.mono}`}>
              {serviceQuery.data.serviceKey}
            </p>
          ) : null}
        </div>
        <div className={styles.buttonRow}>
          {isEdit && serviceQuery.data ? (
            <StatusPill
              label={serviceQuery.data.enabled ? "启用" : "停用"}
              tone={serviceQuery.data.enabled ? "success" : "default"}
            />
          ) : null}
          <ActionButton
            label="返回服务"
            onClick={() => navigate("/services")}
            variant="ghost"
          />
        </div>
      </header>

      {isEdit && serviceQuery.data ? (
        <>
          <QueryRefreshNotice
            query={serviceQuery}
            onRetry={() => void serviceQuery.refetch()}
          />
          <nav className={styles.tabs} aria-label="服务详情">
            <Link
              to={`?tab=overview`}
              className={`${styles.tab} ${activeTab === "overview" ? styles.tabActive : ""}`}
              aria-current={activeTab === "overview" ? "page" : undefined}
            >
              概览
            </Link>
            <Link
              to={`?tab=config`}
              className={`${styles.tab} ${activeTab === "config" ? styles.tabActive : ""}`}
              aria-current={activeTab === "config" ? "page" : undefined}
            >
              配置
            </Link>
          </nav>
        </>
      ) : null}

      <div hidden={activeTab !== "config"}>
        <ServiceForm
          key={serviceId ?? "new"}
          service={serviceQuery.data}
          isEdit={isEdit}
          submitError={submitError}
          savePending={saveMutation.isPending}
          deletePending={deleteMutation.isPending}
          onSubmit={(values) => {
            setSubmitError(null);
            return saveMutation.mutateAsync(values);
          }}
          onDelete={() => void handleDelete()}
          onCancel={() => navigate(isEdit ? "?tab=overview" : "/services")}
        />
      </div>

      {isEdit && serviceQuery.data ? (
        <div hidden={activeTab !== "overview"}>
          <div className={styles.page}>
            <ServiceSummary service={serviceQuery.data} />
            <ServiceReleaseCard
              key={`${serviceQuery.data.id}:${serviceQuery.data.imageRepo}`}
              service={serviceQuery.data}
              disabled={saveMutation.isPending || deleteMutation.isPending}
            />
            <ServiceObservability serviceId={serviceId!} />
          </div>
        </div>
      ) : null}
    </div>
  );
}
