import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router-dom";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { StatusPill } from "../../../shared/components/StatusPill";
import { EmptyState, ErrorState, LoadingState } from "../../../shared/components/StateBlocks";
import { servicesApi } from "../api";
import type { ServiceFormValues } from "../../../shared/lib/forms";
import { toServicePayload } from "../../../shared/lib/forms";
import { ServiceForm } from "./ServiceForm";
import { ServiceSummary } from "./ServiceSummary";
import { ServiceObservability } from "./ServiceObservability";
import styles from "../../../styles/admin.module.css";

export function ServiceEditorPage() {
  const params = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [submitError, setSubmitError] = useState<string | null>(null);
  const serviceId = params.id;
  const isEdit = Boolean(serviceId);

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
      return isEdit ? servicesApi.update(serviceId!, payload) : servicesApi.create(payload);
    },
    onSuccess: async (service) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["services"] }),
        queryClient.invalidateQueries({ queryKey: ["overview"] }),
      ]);
      navigate(`/services/${service.id}`, { replace: true });
    },
    onError: (error) => setSubmitError(getErrorMessage(error)),
  });

  const deleteMutation = useMutation({
    mutationFn: async () => {
      if (!serviceId) {
        throw new Error("服务ID不存在");
      }
      return servicesApi.delete(serviceId);
    },
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
        <LoadingState title="正在加载服务详情" message="正在拉取服务配置与运行摘要。" />
      </div>
    );
  }

  if (isEdit && serviceQuery.isError) {
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

  const handleDelete = () => {
    if (!serviceQuery.data) {
      return;
    }
    if (!window.confirm(`确认删除服务 ${serviceQuery.data.serviceKey}？将同步清理容器与代理配置。`)) {
      return;
    }
    setSubmitError(null);
    deleteMutation.mutate();
  };

  return (
    <div className={styles.page}>
      <section className={styles.sectionHeader}>
        <div>
          <h1 className={styles.sectionTitle}>{isEdit ? "服务详情" : "新建服务"}</h1>
          <p className={styles.sectionCopy}>{isEdit ? "更新服务配置并同步运行策略。" : "填写服务基础信息并创建配置。"}</p>
        </div>
        {isEdit && serviceQuery.data ? (
          <StatusPill
            label={serviceQuery.data.enabled ? "启用" : "停用"}
            tone={serviceQuery.data.enabled ? "success" : "danger"}
          />
        ) : null}
      </section>

      <ServiceForm
        service={serviceQuery.data}
        isEdit={isEdit}
        submitError={submitError}
        savePending={saveMutation.isPending}
        deletePending={deleteMutation.isPending}
        onSubmit={(values) => {
          setSubmitError(null);
          saveMutation.mutate(values);
        }}
        onDelete={handleDelete}
        onCancel={() => navigate("/services")}
      />

      {isEdit && serviceQuery.data ? (
        <>
          <ServiceSummary service={serviceQuery.data} />
          <ServiceObservability serviceId={serviceId!} />
        </>
      ) : null}
    </div>
  );
}
