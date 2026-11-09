import { useEffect, useState } from "react";
import { Download, RefreshCw } from "lucide-react";
import { api } from "./api";
import type { Release, UpdateChannel } from "./types";
import { Notice, useTask } from "./ui";
export function UpdatePanel() {
  const [channel, setChannel] = useState<UpdateChannel>({
    url: "",
    publicKey: "",
  });
  const [release, setRelease] = useState<Release | null>(null);
  const [path, setPath] = useState("");
  const task = useTask();
  useEffect(() => {
    let alive = true;
    void task.run(async () => {
      const saved = await api().UpdateChannel();
      if (alive) setChannel(saved);
    });
    return () => {
      alive = false;
    };
  }, [task.run]);
  return (
    <section className="panel form section-space">
      <h2>
        <RefreshCw size={20} />
        Cập nhật ứng dụng
      </h2>
      <p className="hint">
        Phiên bản 0.1.0. Kênh phát hành chưa được cấu hình sẵn. Bản tải về phải
        có chữ ký Ed25519 và SHA256 hợp lệ. App sao lưu dữ liệu trước khi tải;
        đóng app rồi mở installer để cài phiên bản mới.
      </p>
      <details>
        <summary>Cấu hình kênh phát hành</summary>
        <div className="form">
          <label>
            URL manifest HTTPS
            <input
              value={channel.url}
              onChange={(e) => setChannel({ ...channel, url: e.target.value })}
              placeholder="https://domain-cua-ban/releases/latest.json"
            />
          </label>
          <label>
            Public key Ed25519 (base64)
            <input
              value={channel.publicKey}
              onChange={(e) =>
                setChannel({ ...channel, publicKey: e.target.value })
              }
            />
          </label>
          <button
            disabled={task.busy}
            onClick={() => {
              void task.run(
                () => api().SaveUpdateChannel(channel),
                "Đã lưu kênh cập nhật.",
              );
            }}
          >
            Lưu kênh
          </button>
        </div>
      </details>
      <div className="actions">
        <button
          disabled={task.busy || !channel.url}
          onClick={() => {
            void task.run(async () => {
              await api().SaveUpdateChannel(channel);
              setRelease(await api().CheckUpdate());
            });
          }}
        >
          Kiểm tra bản mới
        </button>
        {release && release.version !== "0.1.0" && (
          <button
            disabled={task.busy}
            onClick={() => {
              void task.run(
                async () => setPath(await api().DownloadUpdate()),
                "Đã tải và xác minh installer.",
              );
            }}
          >
            <Download size={16} />
            Tải bản {release.version}
          </button>
        )}
      </div>
      {release && (
        <p>
          {release.version === "0.1.0"
            ? "Bạn đang dùng phiên bản mới nhất trên kênh này."
            : `Phiên bản trên kênh: ${release.version}`}
          <br />
          {release.notes}
        </p>
      )}
      {path && (
        <label>
          Installer đã xác minh
          <input readOnly value={path} onFocus={(e) => e.target.select()} />
        </label>
      )}
      <Notice message={task.message} failed={task.failed} />
    </section>
  );
}
