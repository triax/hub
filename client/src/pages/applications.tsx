import type { ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";
import Layout from "../../components/layout";
import Application, { ApplicationStep } from "../../models/Application";
import ApplicationRepo from "../../repository/ApplicationRepo";
import { useAppContext } from "../context";

type Filter = "pending" | "all";

export default function ApplicationsPage() {
  const { myself } = useAppContext();
  const repo = useMemo(() => new ApplicationRepo(), []);
  const [applications, setApplications] = useState<Application[]>([]);
  const [filter, setFilter] = useState<Filter>("pending");
  const [loading, setLoading] = useState(false);
  const [updatingID, setUpdatingID] = useState("");
  const [error, setError] = useState("");

  const isWaitingMyself = !myself?.slack?.id || myself.slack.id === "xxx";
  const isAdmin = myself?.slack?.is_admin;

  useEffect(() => {
    if (isWaitingMyself || !isAdmin) return;
    setLoading(true);
    repo.list("onboarding")
      .then(res => setApplications(res.applications ?? []))
      .catch(err => setError(err instanceof Error ? err.message : "取得に失敗しました。"))
      .finally(() => setLoading(false));
  }, [isWaitingMyself, isAdmin, repo]);

  const replaceApplication = (app: Application) => {
    setApplications(prev => prev.map(item => item.id === app.id ? app : item));
  };

  const toggleStep = async (app: Application, step: ApplicationStep) => {
    const steps = app.steps.map(item => (
      item.key === step.key ? { ...item, done: !item.done } : item
    ));
    setUpdatingID(app.id);
    setError("");
    try {
      const updated = await repo.update(app.id, { steps });
      replaceApplication(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "更新に失敗しました。");
    } finally {
      setUpdatingID("");
    }
  };

  const markDone = async (app: Application) => {
    setUpdatingID(app.id);
    setError("");
    try {
      const updated = await repo.update(app.id, { done: true });
      replaceApplication(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : "更新に失敗しました。");
    } finally {
      setUpdatingID("");
    }
  };

  const visibleApplications = filter === "pending"
    ? applications.filter(app => !app.done)
    : applications;

  if (isWaitingMyself) {
    return (
      <Layout>
        <div className="py-10 text-center text-sm text-gray-500">読み込み中...</div>
      </Layout>
    );
  }

  if (!isAdmin) {
    return (
      <Layout>
        <div className="rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700">
          管理者のみアクセスできます。
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="space-y-5">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h1 className="text-xl font-bold text-gray-900">入部申請</h1>
            <p className="mt-1 text-sm text-gray-500">オンボーディング対応状況</p>
          </div>
          <div className="inline-flex rounded-md border border-gray-300 bg-white p-1">
            <FilterButton active={filter === "pending"} onClick={() => setFilter("pending")}>
              未対応
            </FilterButton>
            <FilterButton active={filter === "all"} onClick={() => setFilter("all")}>
              すべて
            </FilterButton>
          </div>
        </div>

        {error && (
          <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            {error}
          </div>
        )}

        {loading ? (
          <div className="py-10 text-center text-sm text-gray-500">読み込み中...</div>
        ) : visibleApplications.length === 0 ? (
          <div className="rounded-md border border-gray-200 bg-white p-6 text-center text-sm text-gray-500">
            表示する申請はありません。
          </div>
        ) : (
          <div className="space-y-3">
            {visibleApplications.map(app => (
              <ApplicationRow
                key={app.id}
                app={app}
                disabled={updatingID === app.id}
                onToggleStep={step => toggleStep(app, step)}
                onMarkDone={() => markDone(app)}
              />
            ))}
          </div>
        )}
      </div>
    </Layout>
  );
}

function FilterButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      className={[
        "rounded px-3 py-1.5 text-sm font-medium",
        active ? "bg-gray-900 text-white" : "text-gray-600 hover:bg-gray-100",
      ].join(" ")}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

function ApplicationRow({
  app,
  disabled,
  onToggleStep,
  onMarkDone,
}: {
  app: Application;
  disabled: boolean;
  onToggleStep: (step: ApplicationStep) => void;
  onMarkDone: () => void;
}) {
  const role = app.fields?.role || "未設定";
  const allStepsDone = app.steps.length > 0 && app.steps.every(step => step.done);

  return (
    <article className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-base font-semibold text-gray-900">{app.name}</h2>
            <span className="rounded bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
              {role}
            </span>
            {app.done && (
              <span className="rounded bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700">
                対応済み
              </span>
            )}
          </div>
          <div className="mt-1 break-all text-sm text-gray-500">{app.email}</div>
          <div className="mt-1 text-xs text-gray-400">{formatDate(app.created_at)}</div>
        </div>
        {allStepsDone && !app.done && (
          <button
            type="button"
            className="rounded-md bg-blue-700 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50"
            onClick={onMarkDone}
            disabled={disabled}
          >
            対応済みにする
          </button>
        )}
      </div>

      <div className="mt-4 space-y-2">
        {app.steps.map(step => (
          <label key={step.key} className="flex items-center gap-3 text-sm text-gray-800">
            <input
              type="checkbox"
              className="h-5 w-5 rounded border-gray-300"
              checked={step.done}
              onChange={() => onToggleStep(step)}
              disabled={disabled}
            />
            <span>{step.label}</span>
          </label>
        ))}
      </div>
    </article>
  );
}

function formatDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleString("ja-JP", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}
