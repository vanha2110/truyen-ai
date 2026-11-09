import { useCallback, useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { AlertCircle, LoaderCircle, X } from "lucide-react";
import type { ChapterStatus, JobStatus } from "./types";

export function useTask() {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [failed, setFailed] = useState(false);
  const pending = useRef(false);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const run = useCallback(async (work: () => Promise<void>, success = "") => {
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    setMessage("");
    try {
      await work();
      if (mounted.current) {
        setFailed(false);
        setMessage(success);
      }
    } catch (error) {
      if (mounted.current) {
        setFailed(true);
        setMessage(
          error instanceof Error
            ? error.message
            : typeof error === "string"
              ? error
              : "Thao tác không thành công",
        );
      }
    } finally {
      pending.current = false;
      if (mounted.current) setBusy(false);
    }
  }, []);
  return { busy, message, failed, run, setMessage };
}
const labels: Record<ChapterStatus | JobStatus, string> = {
  pending: "Chờ biên tập",
  queued: "Trong hàng đợi",
  processing: "Đang biên tập",
  done: "Hoàn tất",
  needs_review: "Cần xem lại",
  error: "Có lỗi",
  running: "Đang chạy",
  paused: "Tạm dừng",
  interrupted: "Chờ tiếp tục",
  cancelled: "Đã hủy",
};
export function Badge({ status }: { status: ChapterStatus | JobStatus }) {
  return <span className={`badge ${status}`}>{labels[status]}</span>;
}
export function Notice({
  message,
  failed = false,
}: {
  message: string;
  failed?: boolean;
}) {
  return message ? (
    <div
      role={failed ? "alert" : "status"}
      className={`notice ${failed ? "error" : ""}`}
    >
      <AlertCircle size={17} />
      <span>{message}</span>
    </div>
  ) : null;
}
export function Loading() {
  return (
    <div className="loading">
      <LoaderCircle size={22} className="spin" />
      Đang tải…
    </div>
  );
}
export function Heading({
  eyebrow,
  title,
  children,
  actions,
}: {
  eyebrow: string;
  title: string;
  children?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <header className="page-heading">
      <div>
        <div className="eyebrow">{eyebrow}</div>
        <h1>{title}</h1>
        {children && <p>{children}</p>}
      </div>
      {actions && <div className="actions">{actions}</div>}
    </header>
  );
}
export function Empty({
  icon,
  title,
  children,
}: {
  icon: ReactNode;
  title: string;
  children: ReactNode;
}) {
  return (
    <div className="empty">
      <div className="empty-icon">{icon}</div>
      <h2>{title}</h2>
      <div>{children}</div>
    </div>
  );
}
export function Modal({
  title,
  children,
  close,
}: {
  title: string;
  children: ReactNode;
  close: () => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = ref.current;
    dialog?.showModal();
    return () => dialog?.close();
  }, []);
  return (
    <dialog ref={ref} onCancel={close}>
      <header>
        <h2>{title}</h2>
        <button className="icon" onClick={close} aria-label="Đóng">
          <X size={20} />
        </button>
      </header>
      {children}
    </dialog>
  );
}
export function duration(seconds: number) {
  if (seconds < 60) return `${Math.round(seconds)} giây`;
  if (seconds < 3600) return `${Math.round(seconds / 60)} phút`;
  return `${(seconds / 3600).toFixed(1)} giờ`;
}
