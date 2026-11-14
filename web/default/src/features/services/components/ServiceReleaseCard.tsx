import { useState } from "react";
import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { ActionButton } from "../../../shared/components/ActionButton";
import { InlineNotice } from "../../../shared/components/StateBlocks";
import { getErrorMessage } from "../../../shared/lib/api-client";
import { releasesApi } from "../../releases/api";
import { servicesApi } from "../api";
import type { ServiceRecord } from "../types";
import styles from "../../../styles/admin.module.css";

interface ServiceReleaseCardProps {
  service: ServiceRecord;
  disabled: boolean;
}

export function ServiceReleaseCard({ service, disabled }: ServiceReleaseCardProps) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [filter, setFilter] = useState("");
  const [imageTag, setImageTag] = useState("");
  const [releaseNotes, setReleaseNotes] = useState("");
  const queryKey = ["service-image-tags", service.id, service.imageRepo];
  const tagsQuery = useInfiniteQuery({
    queryKey,
    initialPageParam: "",
    queryFn: ({ pageParam, signal }) =>
      servicesApi.listImageTags({ serviceId: service.id, cursor: pageParam || undefined }, signal),
    getNextPageParam: (page) => page.nextCursor || undefined,
    refetchOnWindowFocus: false,
    retry: false,
  });
  const createMutation = useMutation({
    mutationFn: () =>
      releasesApi.create({
        serviceId: service.id,
        imageTag,
        releaseNotes: releaseNotes.trim() || undefined,
      }),
    onSuccess: async (release) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["releases"] }),
        queryClient.invalidateQueries({ queryKey: ["overview"] }),
      ]);
      navigate(`/releases/${release.id}`);
    },
  });
  const tags = [...new Set(tagsQuery.data?.pages.flatMap((page) => page.tags) ?? [])];
  const filteredTags = tags.filter((tag) => tag !== "latest" && tag.toLowerCase().includes(filter.toLowerCase()));
  const controlsDisabled = disabled || createMutation.isPending;

  const refreshTags = () => {
    setImageTag("");
    createMutation.reset();
    void queryClient.resetQueries({ queryKey, exact: true });
  };

  return (
    <section className={styles.sectionCard}>
      <div className={styles.sectionHeader}>
        <h2 className={styles.releaseTitle}>主动发布</h2>
        <ActionButton label="刷新 Tag" pending={tagsQuery.isFetching && !tagsQuery.isFetchingNextPage} disabled={controlsDisabled} onClick={refreshTags} />
      </div>

      <form
        className={styles.releaseForm}
        onSubmit={(event) => {
          event.preventDefault();
          createMutation.mutate();
        }}
      >
        <div className={styles.field}>
          <span className={styles.label}>已保存镜像仓库</span>
          <span className={styles.releaseRepository}>{service.imageRepo}</span>
        </div>

        <div className={styles.fieldGrid}>
          <label className={styles.field}>
            <span className={styles.label}>筛选已加载 Tag</span>
            <input
              className={styles.input}
              disabled={controlsDisabled}
              onChange={(event) => {
                const value = event.target.value;
                setFilter(value);
                if (imageTag !== "latest" && !imageTag.toLowerCase().includes(value.toLowerCase())) {
                  setImageTag("");
                }
              }}
              type="search"
              value={filter}
            />
          </label>
          <label className={styles.field}>
            <span className={styles.label}>Tag</span>
            <select className={styles.select} disabled={controlsDisabled} onChange={(event) => setImageTag(event.target.value)} required value={imageTag}>
              <option value="">选择 Tag</option>
              <option value="latest">latest</option>
              {filteredTags.map((tag) => (
                <option key={tag} value={tag}>{tag}</option>
              ))}
            </select>
          </label>
        </div>

        <div className={styles.buttonRow}>
          <span aria-live="polite" className={styles.hint}>
            {tagsQuery.isPending ? "正在加载 Tag" : `已加载 ${tags.length} 个 Tag`}
            {!tagsQuery.isPending && filter && !filteredTags.length ? " · 暂无匹配" : ""}
          </span>
          {tagsQuery.hasNextPage ? (
            <ActionButton
              label={tagsQuery.isFetchNextPageError ? "重试加载" : "加载更多"}
              pending={tagsQuery.isFetchingNextPage}
              disabled={controlsDisabled || tagsQuery.isFetching}
              onClick={() => void tagsQuery.fetchNextPage()}
              variant="ghost"
            />
          ) : null}
          {tagsQuery.isError && !tagsQuery.isFetchNextPageError ? (
            <ActionButton label="重试" disabled={controlsDisabled || tagsQuery.isFetching} onClick={() => void tagsQuery.refetch()} variant="ghost" />
          ) : null}
        </div>

        {tagsQuery.isError ? <InlineNotice message={`Tag 获取失败：${getErrorMessage(tagsQuery.error)}`} tone="error" /> : null}

        <label className={styles.field}>
          <span className={styles.label}>发布说明（可选）</span>
          <textarea className={`${styles.textarea} ${styles.releaseNotes}`} disabled={controlsDisabled} onChange={(event) => setReleaseNotes(event.target.value)} rows={2} value={releaseNotes} />
        </label>

        {createMutation.isError ? <InlineNotice message={getErrorMessage(createMutation.error)} tone="error" /> : null}

        <div className={styles.buttonRow}>
          <ActionButton
            label="创建发布单"
            pending={createMutation.isPending}
            pendingLabel="创建中"
            disabled={disabled || !service.enabled || !imageTag}
            type="submit"
            variant="primary"
          />
          {!service.enabled ? <span className={styles.hint}>服务已停用</span> : null}
        </div>
      </form>
    </section>
  );
}
