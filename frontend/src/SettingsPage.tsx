import { useCallback, useEffect, useState } from "react";
import {
  Activity,
  Database,
  Download,
  RefreshCw,
  Save,
  Server,
  SlidersHorizontal,
} from "lucide-react";
import { api, download } from "./api";
import type { Benchmark, Catalog, Config, Preset } from "./types";
import { Heading, Loading, Notice, duration, useTask } from "./ui";
import { UpdatePanel } from "./UpdatePanel";

interface PullProgress {
  status: string;
  digest: string;
  total: number;
  completed: number;
  error: string;
}
declare global {
  interface Window {
    runtime?: {
      EventsOn(
        name: "model:pull",
        callback: (progress: PullProgress) => void,
      ): () => void;
    };
  }
}
const sampleText =
  "Lâm Phong đứng trước sơn môn, trong lòng có chút thấp thỏm. Hắn nhìn về phía lão giả đang lặng lẽ chờ đợi.\n\nLão giả vuốt râu, chậm rãi nói rằng muốn nhập môn thì trước hết phải vượt qua khảo nghiệm. Lâm Phong gật đầu, bước lên phía trước.";
export function SettingsPage() {
  const [config, setConfig] = useState<Config | null>(null);
  const [catalog, setCatalog] = useState<Catalog | null>(null);
  const [connectionError, setConnectionError] = useState("");
  const [presets, setPresets] = useState<Preset[]>([]);
  const [presetName, setPresetName] = useState("");
  const [pullName, setPullName] = useState("");
  const [pull, setPull] = useState<PullProgress | null>(null);
  const [sample, setSample] = useState(sampleText);
  const [benchmark, setBenchmark] = useState<Benchmark | null>(null);
  const task = useTask();
  const pullTask = useTask();
  useEffect(() => {
    let alive = true;
    void task.run(async () => {
      const [c, p] = await Promise.all([api().Config(), api().Presets()]);
      if (alive) {
        setConfig(c);
        setPresets(p);
      }
    });
    return () => {
      alive = false;
    };
  }, [task.run]);
  const host = config?.ollamaUrl;
  const refreshCatalog = useCallback(async () => {
    if (!host) return;
    try {
      setCatalog(await api().Catalog(host));
      setConnectionError("");
    } catch (error) {
      setCatalog(null);
      setConnectionError(
        error instanceof Error ? error.message : String(error),
      );
    }
  }, [host]);
  useEffect(() => {
    const timer = setTimeout(() => {
      void refreshCatalog();
    }, 500);
    const focus = () => {
      void refreshCatalog();
    };
    window.addEventListener("focus", focus);
    return () => {
      clearTimeout(timer);
      window.removeEventListener("focus", focus);
    };
  }, [refreshCatalog]);
  useEffect(() => window.runtime?.EventsOn("model:pull", setPull), []);
  if (!config)
    return (
      <>
        <Notice message={task.message} failed={task.failed} />
        <Loading />
      </>
    );
  const set = <K extends keyof Config>(key: K, value: Config[K]) =>
    setConfig({ ...config, [key]: value });
  const selected = catalog?.models.find((m) => m.name === config.editor);
  return (
    <>
      <Heading
        eyebrow="THIẾT LẬP KHÔNG GIAN ĐỌC"
        title="Cài đặt"
        actions={
          <button
            className="primary"
            disabled={task.busy}
            onClick={() => {
              void task.run(
                () => api().SaveConfig(config),
                "Đã lưu. Cấu hình có hiệu lực ở job kế tiếp.",
              );
            }}
          >
            <Save size={16} />
            Lưu cài đặt
          </button>
        }
      >
        Model của bạn, chạy trên máy bạn. Không có danh sách model đóng cứng.
      </Heading>
      <Notice message={task.message} failed={task.failed} />
      <div className="settings-grid">
        <section className="panel form">
          <h2>
            <Server size={20} />
            Kết nối Ollama
          </h2>
          <div className="inline-field">
            <label>
              Địa chỉ server
              <input
                value={config.ollamaUrl}
                onChange={(e) => set("ollamaUrl", e.target.value)}
              />
            </label>
            <button
              onClick={() => {
                void refreshCatalog();
              }}
            >
              <RefreshCw size={16} />
              Kiểm tra
            </button>
          </div>
          {catalog ? (
            <div className="connected">
              <span className="status-dot" />
              Đã kết nối · Ollama {catalog.version}
            </div>
          ) : (
            <>
              <Notice message={connectionError} failed />
              <p className="hint">
                Mở Ollama hoặc chạy <code>ollama serve</code>, rồi kiểm tra lại.{" "}
                <a
                  href="https://ollama.com/download"
                  target="_blank"
                  rel="noreferrer"
                >
                  Tải Ollama
                </a>
              </p>
            </>
          )}
          <h3>Model đã cài</h3>
          {catalog?.models.length === 0 && (
            <p className="hint">Chưa có model. Nhập tên bên dưới để tải.</p>
          )}
          <div className="model-list">
            {catalog?.models.map((m) => (
              <div
                className={`model-card ${!m.completion ? "disabled-model" : ""}`}
                key={m.name}
              >
                <div>
                  <strong>{m.name}</strong>
                  <small>
                    {m.parameters} · {m.quantization} ·{" "}
                    {(m.size / 1e9).toFixed(1)} GB · context{" "}
                    {m.context.toLocaleString()}
                  </small>
                </div>
                <span>
                  {m.completion
                    ? m.loaded
                      ? `GPU ${Math.round(m.gpuRatio * 100)}%`
                      : "Chưa nạp"
                    : "Không biên tập được"}
                </span>
                {m.error && <Notice message={m.error} failed />}
              </div>
            ))}
          </div>
          <div className="inline-field">
            <label>
              Tải model mới
              <input
                value={pullName}
                onChange={(e) => setPullName(e.target.value)}
                placeholder="Tên model từ thư viện Ollama"
              />
            </label>
            <button
              disabled={pullTask.busy || !pullName.trim()}
              onClick={() => {
                void pullTask.run(async () => {
                  setPull(null);
                  await api().PullModel(config.ollamaUrl, pullName);
                  await refreshCatalog();
                }, "Đã tải model.");
              }}
            >
              <Download size={16} />
              Tải model
            </button>
          </div>
          <p className="hint">
            <a
              href="https://ollama.com/library"
              target="_blank"
              rel="noreferrer"
            >
              Mở thư viện model Ollama ↗
            </a>
          </p>
          {pullTask.busy && (
            <div>
              <p>{pull?.status ?? "Đang kết nối…"}</p>
              {pull && pull.total > 0 && (
                <progress max={pull.total} value={pull.completed} />
              )}
              <button
                onClick={() => {
                  void api().CancelPull();
                }}
              >
                Hủy tải
              </button>
            </div>
          )}
          <Notice message={pullTask.message} failed={pullTask.failed} />
        </section>
        <section className="panel form">
          <h2>
            <SlidersHorizontal size={20} />
            Model và tham số
          </h2>
          {(
            [
              { key: "editor", label: "Biên tập" },
              { key: "extractor", label: "Trích xuất tên" },
              { key: "summarizer", label: "Tóm tắt ngữ cảnh" },
            ] satisfies {
              key: "editor" | "extractor" | "summarizer";
              label: string;
            }[]
          ).map(({ key, label }) => (
            <label key={key}>
              {label}
              <select
                value={config[key]}
                onChange={(e) => set(key, e.target.value)}
              >
                <option value="">
                  {key === "editor" ? "Chọn model" : "Chưa sử dụng"}
                </option>
                {config[key] &&
                  !catalog?.models.some((m) => m.name === config[key]) && (
                    <option value={config[key]}>
                      {config[key]} (chưa tìm thấy)
                    </option>
                  )}
                {catalog?.models
                  .filter((m) => m.completion)
                  .map((m) => (
                    <option key={m.name} value={m.name}>
                      {m.name}
                    </option>
                  ))}
              </select>
            </label>
          ))}
          <div className="form-grid">
            {(
              [
                {
                  key: "temperature",
                  label: "Temperature",
                  min: 0,
                  max: 2,
                  step: 0.1,
                },
                { key: "topP", label: "Top P", min: 0.01, max: 1, step: 0.05 },
                {
                  key: "repeatPenalty",
                  label: "Repeat penalty",
                  min: 1,
                  max: 2,
                  step: 0.05,
                },
                {
                  key: "numCtx",
                  label: "Context (num_ctx)",
                  min: 2048,
                  max: selected?.context || 1048576,
                  step: 1024,
                },
                {
                  key: "numPredict",
                  label: "Output tối đa",
                  min: 128,
                  max: config.numCtx - 1,
                  step: 128,
                },
                {
                  key: "chunkWords",
                  label: "Từ mỗi chunk",
                  min: 50,
                  max: 2000,
                  step: 50,
                },
                {
                  key: "retries",
                  label: "Số lần retry",
                  min: 0,
                  max: 3,
                  step: 1,
                },
              ] satisfies {
                key:
                  | "temperature"
                  | "topP"
                  | "repeatPenalty"
                  | "numCtx"
                  | "numPredict"
                  | "chunkWords"
                  | "retries";
                label: string;
                min: number;
                max: number;
                step: number;
              }[]
            ).map(({ key, label, ...limits }) => (
              <label key={key}>
                {label}
                <input
                  type="number"
                  {...limits}
                  value={config[key]}
                  onChange={(e) => set(key, Number(e.target.value))}
                />
              </label>
            ))}
            <label>
              Keep alive
              <input
                value={config.keepAlive}
                onChange={(e) => set("keepAlive", e.target.value)}
              />
            </label>
          </div>
          <label className="check">
            <input
              type="checkbox"
              checked={config.thinkOff}
              onChange={(e) => set("thinkOff", e.target.checked)}
            />
            Tắt thinking khi model hỗ trợ
          </label>
          <div className="inline-field">
            <label>
              Preset
              <select
                defaultValue=""
                onChange={(e) => {
                  const p = presets.find((v) => v.name === e.target.value);
                  if (p) setConfig(p.config);
                }}
              >
                <option value="">Chọn bộ tham số đã lưu</option>
                {presets.map((p) => (
                  <option key={p.name}>{p.name}</option>
                ))}
              </select>
            </label>
          </div>
          <div className="inline-field">
            <input
              aria-label="Tên preset mới"
              placeholder="Tên preset mới"
              value={presetName}
              onChange={(e) => setPresetName(e.target.value)}
            />
            <button
              disabled={!presetName || task.busy}
              onClick={() => {
                void task.run(async () => {
                  await api().SavePreset(presetName, config);
                  setPresets(await api().Presets());
                }, "Đã lưu preset.");
              }}
            >
              Lưu preset
            </button>
          </div>
        </section>
      </div>
      <section className="panel form section-space">
        <h2>
          <Activity size={20} />
          Thử model
        </h2>
        <p className="hint">
          Đo tốc độ và kiểm tra cấu trúc đoạn trên văn bản thật. Ước tính thời
          gian dựa trên giả định 2.000 từ/chương.
        </p>
        <textarea
          rows={5}
          value={sample}
          onChange={(e) => setSample(e.target.value)}
        />
        <button
          className="primary"
          disabled={task.busy || !config.editor}
          onClick={() => {
            void task.run(async () =>
              setBenchmark(await api().Benchmark(config, sample)),
            );
          }}
        >
          <Activity size={16} />
          {task.busy ? "Đang xử lý…" : "Chạy benchmark"}
        </button>
        {benchmark && (
          <>
            <div className="benchmark-stats">
              <span>
                <strong>{benchmark.tokensPerSecond.toFixed(1)}</strong>{" "}
                token/giây
              </span>
              <span>
                <strong>{Math.round(benchmark.gpuRatio * 100)}%</strong> trên
                GPU
              </span>
              <span>
                <strong>{benchmark.loadMs}</strong> ms nạp model
              </span>
              <span>
                50 chương: <strong>{duration(benchmark.estimated50)}</strong>
              </span>
              <span>
                100 chương: <strong>{duration(benchmark.estimated100)}</strong>
              </span>
            </div>
            {benchmark.gpuRatio < 0.99 && (
              <Notice message="Model đang dùng thêm RAM/CPU. Có thể thử model nhỏ hơn hoặc giảm num_ctx." />
            )}
            <Notice
              message={
                benchmark.result.flags.length
                  ? `Cần xem lại: ${benchmark.result.flags.join(", ")}`
                  : `Qua kiểm tra cấu trúc · tỷ lệ độ dài ${benchmark.result.ratio.toFixed(2)}. Đọc lại để đánh giá độ đúng nghĩa và mượt.`
              }
              failed={benchmark.result.flags.length > 0}
            />
            <pre className="benchmark-output">{benchmark.result.text}</pre>
          </>
        )}
      </section>
      <section className="panel form section-space">
        <h2>
          <Database size={20} />
          Hàng đợi và dữ liệu
        </h2>
        <label className="check">
          <input
            type="checkbox"
            checked={config.autoResume}
            onChange={(e) => set("autoResume", e.target.checked)}
          />
          Tự tiếp tục các job bị gián đoạn khi mở app
        </label>
        <label className="check">
          <input
            type="checkbox"
            checked={config.nightOnly}
            onChange={(e) => set("nightOnly", e.target.checked)}
          />
          Chỉ chạy trong khung giờ đã chọn (app cần đang mở)
        </label>
        {config.nightOnly && (
          <div className="form-grid">
            <label>
              Bắt đầu (0–23)
              <input
                type="number"
                min={0}
                max={23}
                value={config.nightStart}
                onChange={(e) => set("nightStart", Number(e.target.value))}
              />
            </label>
            <label>
              Kết thúc (0–23)
              <input
                type="number"
                min={0}
                max={23}
                value={config.nightEnd}
                onChange={(e) => set("nightEnd", Number(e.target.value))}
              />
            </label>
          </div>
        )}
        <label className="check">
          <input
            type="checkbox"
            checked={config.autostart}
            onChange={(e) => set("autostart", e.target.checked)}
          />
          Khởi động cùng Windows, chạy dưới khay hệ thống
        </label>
        <label className="check">
          <input
            type="checkbox"
            checked={config.notifications}
            onChange={(e) => set("notifications", e.target.checked)}
          />
          Thông báo Windows khi hoàn tất job
        </label>
        <label>
          Biên tập trước khi đọc (0 = tắt)
          <input
            type="number"
            min={0}
            max={10}
            value={config.prefetch}
            onChange={(e) => set("prefetch", Number(e.target.value))}
          />
        </label>
        <p className="hint">
          Sao lưu tự động sau mỗi job; giữ 5 bản gần nhất. Dữ liệu mặc định tại
          %AppData%\ConvertFix. Có thể dùng CONVERTFIX_DATA để chọn thư mục khác
          trước khi mở app.
        </p>
        <div className="actions">
          <button
            disabled={task.busy}
            onClick={() => {
              void task.run(async () => {
                const path = await api().Backup();
                window.alert(`Đã sao lưu: ${path}`);
              });
            }}
          >
            Sao lưu ngay
          </button>
          <button
            onClick={() =>
              download(
                "convertfix-config.json",
                JSON.stringify(config, null, 2),
              )
            }
          >
            Xuất cấu hình
          </button>
          <label className="button">
            Nhập cấu hình
            <input
              hidden
              type="file"
              accept=".json"
              disabled={task.busy}
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file)
                  void task.run(async () => {
                    if (file.size > 64000)
                      throw new Error("Tệp cấu hình quá lớn");
                    await api().ImportConfig(await file.text());
                    setConfig(await api().Config());
                  }, "Đã nhập cấu hình. Thiết lập khởi động cùng Windows được giữ nguyên.");
                e.target.value = "";
              }}
            />
          </label>
          <button
            disabled={task.busy}
            onClick={() => {
              void task.run(async () =>
                download(
                  "convertfix-diagnostics.json",
                  await api().Diagnostics(),
                ),
              );
            }}
          >
            Xuất báo cáo chẩn đoán
          </button>
        </div>
      </section>
      <UpdatePanel />
    </>
  );
}
