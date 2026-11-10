import { useEffect, useState } from "react";
import {
  ArrowLeft,
  BookA,
  BookOpen,
  FileText,
  Globe,
  ListOrdered,
  Pause,
  Play,
  Plus,
  RotateCcw,
  Square,
  Upload,
} from "lucide-react";
import { api } from "./api";
import { defaultSource } from "./types";
import type {
  Chapter,
  Novel,
  Preview,
  Route,
  Source,
  State,
  Style,
} from "./types";
import {
  Badge,
  Empty,
  Heading,
  Loading,
  Modal,
  Notice,
  duration,
  useTask,
} from "./ui";

interface Navigate {
  navigate: (route: Route) => void;
  refresh: () => Promise<void>;
}
export function AddPage({ navigate, refresh }: Navigate) {
  const [mode, setMode] = useState<"url" | "text">("url");
  const [url, setUrl] = useState("");
  const [source, setSource] = useState<Source>(defaultSource);
  const [title, setTitle] = useState("");
  const [text, setText] = useState("");
  const [preview, setPreview] = useState<Preview | null>(null);
  const [range, setRange] = useState({ from: 1, to: 1 });
  const task = useTask();
  const changeSource = (key: keyof Source, value: string) => {
    setSource((s) => ({ ...s, [key]: value }));
    setPreview(null);
  };
  const inspect = () =>
    task.run(async () => {
      const result = await api().Preview(url, source);
      setPreview(result);
      setTitle(result.novel.title);
      setRange({
        from: result.chapters[0]?.index ?? 1,
        to: result.chapters.at(-1)?.index ?? 1,
      });
    });
  const add = () =>
    task.run(async () => {
      let id: number;
      if (mode === "text") id = await api().ImportText(title, text);
      else {
        if (!preview)
          throw new Error("Xem trước mục lục trước khi thêm truyện");
        const chapters = preview.chapters.filter(
          (ch) => ch.index >= range.from && ch.index <= range.to,
        );
        if (!chapters.length) throw new Error("Khoảng chọn không có chương");
        id = await api().AddSource(
          { ...preview, novel: { ...preview.novel, title }, chapters },
          source,
        );
      }
      await refresh();
      navigate({ kind: "novel", novelId: id });
    });
  return (
    <>
      <Heading eyebrow="BẮT ĐẦU MỘT CÂU CHUYỆN" title="Thêm vào thư viện">
        Tự chọn nguồn truyện. Bản gốc được lưu riêng và luôn có thể đọc lại.
      </Heading>
      <div className="narrow">
        <div className="tabs">
          <button
            className={mode === "url" ? "active" : ""}
            onClick={() => setMode("url")}
          >
            <Globe size={17} />
            Từ website
          </button>
          <button
            className={mode === "text" ? "active" : ""}
            onClick={() => setMode("text")}
          >
            <FileText size={17} />
            Nhập văn bản
          </button>
        </div>
        <div className="panel form">
          <Notice message={task.message} failed={task.failed} />
          {mode === "url" ? (
            <>
              <label>
                Đường dẫn mục lục truyện
                <input
                  type="url"
                  value={url}
                  onChange={(e) => {
                    setUrl(e.target.value);
                    setPreview(null);
                  }}
                  placeholder="https://website-cua-ban/truyen/…"
                  disabled={task.busy}
                />
              </label>
              <p className="hint">
                Nhập trang có danh sách chương. App tải chậm và tuân theo
                robots.txt của nguồn.
              </p>
              <details>
                <summary>Tùy chỉnh selector cho nguồn truyện</summary>
                <div className="form-grid">
                  {(
                    [
                      { key: "title", label: "Tên truyện" },
                      { key: "chapterLinks", label: "Liên kết chương" },
                      {
                        key: "content",
                        label: "Nội dung chương (trống = tự tìm)",
                      },
                      {
                        key: "next",
                        label: "Trang mục lục kế tiếp (trống = tự tìm)",
                      },
                      { key: "author", label: "Tác giả" },
                      { key: "description", label: "Mô tả" },
                      { key: "remove", label: "Các phần cần loại bỏ" },
                    ] satisfies { key: keyof Source; label: string }[]
                  ).map(({ key, label }) => (
                    <label key={key}>
                      {label}
                      <input
                        value={source[key]}
                        onChange={(e) => changeSource(key, e.target.value)}
                        disabled={task.busy}
                      />
                    </label>
                  ))}
                </div>
              </details>
              <button
                disabled={task.busy || !url}
                onClick={() => {
                  void inspect();
                }}
              >
                <Globe size={16} />
                {task.busy ? "Đang đọc nguồn…" : "Xem trước mục lục"}
              </button>
              {preview && (
                <div className="source-preview">
                  <h3>{preview.novel.title}</h3>
                  <p>
                    {preview.chapters.length} chương từ {preview.pages} trang
                    mục lục ·{" "}
                    {preview.numbering === "catalog" ? "Số thứ tự" : "Chương"}{" "}
                    {preview.chapters[0]?.index}–
                    {preview.chapters.at(-1)?.index}
                  </p>
                  {preview.warnings.map((w) => (
                    <Notice key={w} message={w} />
                  ))}
                  <div className="form-grid">
                    <label>
                      {preview.numbering === "catalog"
                        ? "Từ số thứ tự"
                        : "Từ chương"}
                      <input
                        type="number"
                        min={1}
                        value={range.from}
                        onChange={(e) =>
                          setRange({ ...range, from: Number(e.target.value) })
                        }
                      />
                    </label>
                    <label>
                      {preview.numbering === "catalog"
                        ? "Đến số thứ tự"
                        : "Đến chương"}
                      <input
                        type="number"
                        min={range.from}
                        value={range.to}
                        onChange={(e) =>
                          setRange({ ...range, to: Number(e.target.value) })
                        }
                      />
                    </label>
                  </div>
                  <div className="preview-list">
                    {preview.chapters.slice(0, 25).map((ch) => (
                      <div key={ch.index}>
                        {preview.numbering === "catalog" ? `${ch.index}. ` : ""}
                        {ch.title}
                      </div>
                    ))}
                    {preview.chapters.length > 25 && (
                      <small>
                        … và {preview.chapters.length - 25} chương khác
                      </small>
                    )}
                  </div>
                </div>
              )}
            </>
          ) : (
            <>
              <label className="file-drop">
                <Upload size={22} />
                Chọn tệp .txt UTF-8
                <input
                  type="file"
                  accept=".txt,text/plain"
                  disabled={task.busy}
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    if (file)
                      void task.run(async () => {
                        if (file.size > 32 * 1024 * 1024)
                          throw new Error("Tệp vượt 32 MB");
                        setText(await file.text());
                        setTitle(file.name.replace(/\.txt$/i, ""));
                      });
                  }}
                />
              </label>
              <label>
                Bản convert
                <textarea
                  rows={13}
                  value={text}
                  onChange={(e) => setText(e.target.value)}
                  placeholder={
                    "Chương 1: Khởi đầu\n\nDán nội dung truyện ở đây…"
                  }
                />
              </label>
              <p className="hint">
                Tách chương theo dòng “Chương 1…”. Nếu không có tiêu đề chương,
                văn bản được nhập thành một chương.
              </p>
            </>
          )}
          <label>
            Tên truyện
            <input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Đặt tên cho câu chuyện"
            />
          </label>
          <div className="form-footer">
            <span>Chọn model và xếp hàng ở bước tiếp theo.</span>
            <button
              className="primary"
              disabled={
                task.busy ||
                !title.trim() ||
                (mode === "url" ? !preview : !text.trim())
              }
              onClick={() => {
                void add();
              }}
            >
              <Plus size={17} />
              Thêm vào thư viện
            </button>
          </div>
        </div>
      </div>
    </>
  );
}

export function NovelPage({
  novel,
  novelId,
  navigate,
  refresh,
}: Navigate & { novel: Novel | undefined; novelId: number }) {
  const [chapters, setChapters] = useState<Chapter[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [range, setRange] = useState({ from: 1, to: 1 });
  const [filter, setFilter] = useState("all");
  const [style, setStyle] = useState<Style | null>(null);
  const [showStyle, setShowStyle] = useState(false);
  const [source, setSource] = useState<Source | null>(null);
  const task = useTask();
  useEffect(() => {
    let alive = true;
    let pending = false;
    const load = async () => {
      if (pending) return;
      pending = true;
      try {
        const list = await api().Chapters(novelId);
        if (alive) {
          setChapters(list);
          setLoaded(true);
        }
      } catch (e) {
        if (alive) task.setMessage(String(e));
      } finally {
        pending = false;
      }
    };
    void task.run(async () => {
      const list = await api().Chapters(novelId);
      if (alive) {
        setChapters(list);
        setLoaded(true);
        setRange({ from: list[0]?.index ?? 1, to: list.at(-1)?.index ?? 1 });
      }
    });
    const timer = setInterval(() => {
      void load();
    }, 3000);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [novelId, task.run, task.setMessage]);
  const read = novel?.lastReadChapter || chapters[0]?.id;
  const usesCatalogOrder = chapters.some((ch) =>
    ch.flags.includes("catalog_position="),
  );
  return (
    <>
      <button className="back" onClick={() => navigate({ kind: "library" })}>
        <ArrowLeft size={16} />
        Thư viện
      </button>
      <Heading
        eyebrow="CÂU CHUYỆN CỦA BẠN"
        title={novel?.title ?? "Chi tiết truyện"}
        actions={
          <>
            {read && (
              <button
                className="primary"
                onClick={() =>
                  navigate({ kind: "reader", novelId, chapterId: read })
                }
              >
                <BookOpen size={17} />
                Tiếp tục đọc
              </button>
            )}
            <button onClick={() => navigate({ kind: "glossary", novelId })}>
              <BookA size={17} />
              Thuật ngữ
            </button>
          </>
        }
      >
        {novel?.author || "Chưa có thông tin tác giả"} · {chapters.length}{" "}
        chương
      </Heading>
      <Notice message={task.message} failed={task.failed} />
      {usesCatalogOrder && (
        <Notice message="Nguồn có nhãn chương bị trùng. Khoảng chọn bên dưới dùng số thứ tự mục lục trong app; tiêu đề nguồn được giữ nguyên để đối chiếu." />
      )}
      <div className="panel inline-form">
        <label>
          {usesCatalogOrder ? "Từ số thứ tự" : "Từ chương"}
          <input
            type="number"
            min={1}
            value={range.from}
            onChange={(e) =>
              setRange({ ...range, from: Number(e.target.value) })
            }
          />
        </label>
        <label>
          {usesCatalogOrder ? "Đến số thứ tự" : "Đến chương"}
          <input
            type="number"
            min={range.from}
            value={range.to}
            onChange={(e) => setRange({ ...range, to: Number(e.target.value) })}
          />
        </label>
        <button
          className="primary"
          disabled={task.busy}
          onClick={() => {
            void task.run(async () => {
              await api().AddJob(novelId, range.from, range.to, 0);
              await refresh();
            }, "Đã thêm job. Chương chưa có nội dung sẽ được tải trước khi biên tập.");
          }}
        >
          <Play size={16} />
          Tải và biên tập
        </button>
        <button
          onClick={() => {
            void task.run(async () => {
              setStyle(await api().Style(novelId));
              setShowStyle(true);
            });
          }}
        >
          Phong cách
        </button>
        {novel?.sourceUrl && (
          <button
            onClick={() => {
              void task.run(async () => setSource(await api().Source(novelId)));
            }}
          >
            Cấu hình nguồn
          </button>
        )}
        <button
          onClick={() => {
            void task.run(async () => {
              const config = await api().Config();
              await api().SaveOverride(novelId, config);
            }, "Đã dùng cấu hình hiện tại làm cấu hình riêng của truyện.");
          }}
        >
          Dùng model hiện tại riêng cho truyện
        </button>
        <button
          onClick={() => {
            void task.run(
              () => api().SaveOverride(novelId, null),
              "Đã trở về cấu hình chung.",
            );
          }}
        >
          Dùng cấu hình chung
        </button>
      </div>
      <div className="section-heading">
        <h2>Danh sách chương</h2>
        <select
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          aria-label="Lọc chương"
        >
          <option value="all">Tất cả trạng thái</option>
          <option value="done">Hoàn tất</option>
          <option value="needs_review">Cần xem lại</option>
          <option value="pending">Chờ biên tập</option>
        </select>
      </div>
      {!loaded ? (
        <Loading />
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>CHƯƠNG</th>
                <th>TRẠNG THÁI</th>
                <th>MODEL</th>
                <th>THAO TÁC</th>
              </tr>
            </thead>
            <tbody>
              {chapters
                .filter((ch) => filter === "all" || ch.status === filter)
                .map((ch) => (
                  <tr key={ch.id}>
                    <td>
                      <button
                        className="text-button"
                        onClick={() =>
                          navigate({
                            kind: "reader",
                            novelId,
                            chapterId: ch.id,
                          })
                        }
                      >
                        {usesCatalogOrder ? `${ch.index}. ` : ""}
                        {ch.title}
                      </button>
                      {ch.manualEdit && (
                        <small className="hint">Đã sửa tay</small>
                      )}
                    </td>
                    <td>
                      <Badge status={ch.status} />
                      {ch.flags && (
                        <small className="flag-text">{ch.flags}</small>
                      )}
                    </td>
                    <td>
                      <small>{ch.modelUsed || "—"}</small>
                    </td>
                    <td>
                      {ch.sourceUrl && (
                        <button
                          className="text-button"
                          disabled={task.busy}
                          onClick={() => {
                            if (
                              window.confirm(
                                `Tải lại nội dung gốc từ website cho “${ch.title}”? Bản biên tập${ch.manualEdit ? " đã sửa tay" : ""} sẽ được đặt lại để biên tập từ đầu. App sao lưu trước khi thay đổi. Hãy hủy job của chương trước khi tải lại.`,
                              )
                            ) {
                              void task.run(async () => {
                                await api().ReloadChapter(ch.id, ch.manualEdit);
                                await refresh();
                              }, "Đã tải lại nội dung gốc. Có thể biên tập lại chương.");
                            }
                          }}
                        >
                          Tải lại gốc
                        </button>
                      )}
                      <button
                        className="icon"
                        title={
                          ch.status === "needs_review"
                            ? "Dịch lại các chunk lỗi"
                            : "Dịch lại chương"
                        }
                        disabled={task.busy}
                        onClick={() => {
                          const confirmed =
                            !ch.manualEdit ||
                            window.confirm(
                              "Chương đã sửa tay. Cho phép biên tập lại và ghi đè bản sửa?",
                            );
                          if (confirmed)
                            void task.run(async () => {
                              await api().RetryChapter(
                                ch.id,
                                ch.status === "needs_review",
                                ch.manualEdit,
                              );
                              await refresh();
                            }, "Đã tạo job dịch lại.");
                        }}
                      >
                        <RotateCcw size={16} />
                      </button>
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
      )}
      {source && (
        <Modal title="Cấu hình nguồn truyện" close={() => setSource(null)}>
          <div className="form">
            <p className="hint">
              Tạm dừng rồi tiếp tục job để dùng selector mới. Nội dung đã tải
              được giữ nguyên; dùng “Tải lại gốc” ở danh sách chương để lấy lại
              nội dung bằng cấu hình mới.
            </p>
            {(
              [
                { key: "content", label: "Selector nội dung chương" },
                { key: "chapterLinks", label: "Selector liên kết chương" },
                { key: "next", label: "Selector trang mục lục kế tiếp" },
                { key: "remove", label: "Selector phần cần loại bỏ" },
              ] satisfies { key: keyof Source; label: string }[]
            ).map(({ key, label }) => (
              <label key={key}>
                {label}
                <input
                  value={source[key]}
                  onChange={(e) =>
                    setSource({ ...source, [key]: e.target.value })
                  }
                />
              </label>
            ))}
            <Notice message={task.message} failed={task.failed} />
            <button
              className="primary"
              disabled={task.busy}
              onClick={() => {
                void task.run(async () => {
                  await api().SaveSource(novelId, source);
                  setSource(null);
                }, "Đã lưu cấu hình nguồn.");
              }}
            >
              Lưu nguồn
            </button>
          </div>
        </Modal>
      )}
      {showStyle && style && (
        <Modal title="Phong cách của truyện" close={() => setShowStyle(false)}>
          <div className="form">
            <label>
              Quy tắc xưng hô
              <textarea
                value={style.pronouns}
                onChange={(e) =>
                  setStyle({ ...style, pronouns: e.target.value })
                }
              />
            </label>
            <label>
              Giọng văn
              <textarea
                value={style.tone}
                onChange={(e) => setStyle({ ...style, tone: e.target.value })}
              />
            </label>
            <button
              className="primary"
              disabled={task.busy}
              onClick={() => {
                void task.run(async () => {
                  await api().SaveStyle(novelId, style);
                  setShowStyle(false);
                }, "Đã lưu phong cách.");
              }}
            >
              Lưu phong cách
            </button>
          </div>
        </Modal>
      )}
    </>
  );
}

export function QueuePage({
  state,
  refresh,
}: {
  state: State;
  refresh: () => Promise<void>;
}) {
  const task = useTask();
  return (
    <>
      <Heading
        eyebrow="CÔNG VIỆC NỀN"
        title="Từng chương, từng bước."
        actions={
          <span className="subtle">Biên tập lần lượt · tự lưu tiến độ</span>
        }
      >
        Tạm dừng bất cứ lúc nào. Mở lại app để tiếp tục từ chunk chưa hoàn tất.
      </Heading>
      <Notice message={task.message} failed={task.failed} />
      {state.jobs.length === 0 ? (
        <Empty icon={<ListOrdered size={34} />} title="Hàng đợi đang trống">
          <p>Chọn khoảng chương trong một truyện để bắt đầu biên tập.</p>
        </Empty>
      ) : (
        <div className="job-list">
          {state.jobs.map((job) => (
            <article className="panel job" key={job.id}>
              <div className="job-top">
                <div>
                  <div className="eyebrow">
                    JOB #{job.id} · CHƯƠNG {job.from}–{job.to}
                  </div>
                  <h2>{job.title}</h2>
                </div>
                <Badge status={job.status} />
              </div>
              <progress value={job.completed} max={Math.max(job.total, 1)} />
              <div className="job-meta">
                <span>
                  {job.completed} / {job.total || "…"} chunk đã lưu
                  {job.total === 0 && " · sẽ tính sau khi tải chương"}
                </span>
                {state.progress.jobId === job.id &&
                  job.status === "running" && (
                    <span>
                      {state.progress.phase === "downloading"
                        ? "Đang tải"
                        : "Đang biên tập"}{" "}
                      chương {state.progress.chapter} ·{" "}
                      {state.progress.tokensPerSecond.toFixed(1)} token/s · ETA{" "}
                      {duration(state.progress.etaSeconds)}
                    </span>
                  )}
              </div>
              {job.error && (
                <Notice
                  message={`${job.error}${job.autoRetry ? " · Sẽ tự kết nối lại." : ""}`}
                  failed
                />
              )}
              <div className="job-actions">
                <label>
                  Ưu tiên
                  <select
                    value={job.priority}
                    disabled={
                      task.busy ||
                      job.status === "done" ||
                      job.status === "cancelled"
                    }
                    onChange={(e) => {
                      void task.run(async () => {
                        await api().Priority(job.id, Number(e.target.value));
                        await refresh();
                      });
                    }}
                  >
                    <option value={0}>Bình thường</option>
                    <option value={1}>Cao</option>
                    <option value={2}>Rất cao</option>
                  </select>
                </label>
                <div className="actions">
                  {!["done", "cancelled"].includes(job.status) && (
                    <>
                      <button
                        disabled={task.busy}
                        onClick={() => {
                          void task.run(async () => {
                            await api().ControlJob(
                              job.id,
                              job.status === "running" ||
                                job.status === "queued" ||
                                job.status === "interrupted" ||
                                job.autoRetry
                                ? "pause"
                                : "resume",
                            );
                            await refresh();
                          });
                        }}
                      >
                        {job.status === "running" ||
                        job.status === "queued" ||
                        job.status === "interrupted" ||
                        job.autoRetry ? (
                          <>
                            <Pause size={15} />
                            Tạm dừng
                          </>
                        ) : (
                          <>
                            <Play size={15} />
                            Tiếp tục
                          </>
                        )}
                      </button>
                      <button
                        className="danger-quiet"
                        disabled={task.busy}
                        onClick={() => {
                          void task.run(async () => {
                            await api().ControlJob(job.id, "cancel");
                            await refresh();
                          });
                        }}
                      >
                        <Square size={14} />
                        Hủy job
                      </button>
                    </>
                  )}
                </div>
              </div>
            </article>
          ))}
        </div>
      )}
    </>
  );
}
