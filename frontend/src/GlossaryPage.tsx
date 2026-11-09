import { useEffect, useState } from "react";
import {
  BookA,
  Check,
  Download,
  Edit3,
  Lock,
  Plus,
  Search,
  Sparkles,
  Trash2,
  Upload,
} from "lucide-react";
import { api, download } from "./api";
import { newTerm } from "./types";
import type { Novel, Replacement, Route, Suggestion, Term } from "./types";
import { Empty, Heading, Modal, Notice, useTask } from "./ui";

export function GlossaryPage({
  novelId,
  selection,
  novels,
  navigate,
}: {
  novelId: number;
  selection: string;
  novels: Novel[];
  navigate: (route: Route) => void;
}) {
  const [terms, setTerms] = useState<Term[]>([]);
  const [search, setSearch] = useState("");
  const [term, setTerm] = useState<Term | null>(
    selection ? newTerm(novelId, selection) : null,
  );
  const [tab, setTab] = useState<"terms" | "suggest" | "learn">("terms");
  const [suggestions, setSuggestions] = useState<Suggestion[]>([]);
  const [raw, setRaw] = useState("");
  const [reference, setReference] = useState("");
  const [changes, setChanges] = useState<Replacement[] | null>(null);
  const [format, setFormat] = useState<"json" | "csv">("json");
  const task = useTask();
  useEffect(() => {
    let alive = true;
    void task.run(async () => {
      const list = await api().Terms(novelId);
      if (alive) setTerms(list);
    });
    return () => {
      alive = false;
    };
  }, [novelId, task.run]);
  const reload = async () => setTerms(await api().Terms(novelId));
  const approve = (p: Suggestion) =>
    task.run(async () => {
      await api().SaveTerm(
        {
          ...newTerm(novelId, p.source),
          target: p.target,
          origin: tab === "learn" ? "reference" : "suggested",
        },
        false,
      );
      setSuggestions((s) => s.filter((v) => v.source !== p.source));
      await reload();
    }, "Đã thêm thuật ngữ.");
  return (
    <>
      <Heading
        eyebrow="NHẤT QUÁN XUYÊN SUỐT CÂU CHUYỆN"
        title="Bảng thuật ngữ"
        actions={
          <button className="primary" onClick={() => setTerm(newTerm(novelId))}>
            <Plus size={17} />
            Thêm thuật ngữ
          </button>
        }
      >
        Tên nhân vật, địa danh và công pháp được chuẩn hóa bằng code sau mỗi lần
        biên tập.
      </Heading>
      <Notice message={task.message} failed={task.failed} />
      <div className="section-heading">
        <label className="scope-label">
          Phạm vi
          <select
            value={novelId}
            onChange={(e) =>
              navigate({ kind: "glossary", novelId: Number(e.target.value) })
            }
          >
            <option value={0}>Toàn bộ thư viện (global)</option>
            {novels.map((n) => (
              <option key={n.id} value={n.id}>
                {n.title}
              </option>
            ))}
          </select>
        </label>
        <label className="search">
          <Search size={17} />
          <input
            placeholder="Tìm tên hoặc thuật ngữ…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </label>
      </div>
      <div className="tabs">
        <button
          className={tab === "terms" ? "active" : ""}
          onClick={() => setTab("terms")}
        >
          <BookA size={16} />
          Thuật ngữ ({terms.length})
        </button>
        <button
          className={tab === "suggest" ? "active" : ""}
          disabled={!novelId}
          onClick={() => {
            setTab("suggest");
            setSuggestions([]);
          }}
        >
          <Sparkles size={16} />
          Gợi ý tên
        </button>
        <button
          className={tab === "learn" ? "active" : ""}
          disabled={!novelId}
          onClick={() => {
            setTab("learn");
            setSuggestions([]);
          }}
        >
          Học từ bản tham chiếu
        </button>
      </div>
      {tab === "terms" ? (
        <>
          <div className="glossary-tools">
            <select
              value={format}
              onChange={(e) => {
                if (e.target.value === "json" || e.target.value === "csv")
                  setFormat(e.target.value);
              }}
              aria-label="Định dạng glossary"
            >
              <option value="json">JSON</option>
              <option value="csv">CSV</option>
            </select>
            <label className="button">
              <Upload size={15} />
              Nhập
              <input
                type="file"
                accept=".json,.csv"
                hidden
                disabled={task.busy}
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file)
                    void task.run(async () => {
                      if (file.size > 8 * 1024 * 1024)
                        throw new Error("Tệp vượt 8 MB");
                      await api().ImportTerms(
                        await file.text(),
                        file.name.endsWith(".csv") ? "csv" : "json",
                        novelId || null,
                      );
                      await reload();
                    }, "Đã nhập glossary.");
                  e.target.value = "";
                }}
              />
            </label>
            <button
              disabled={task.busy}
              onClick={() => {
                void task.run(async () =>
                  download(
                    `glossary-${novelId}.${format}`,
                    await api().ExportTerms(novelId, format),
                  ),
                );
              }}
            >
              <Download size={15} />
              Xuất
            </button>
            {novelId > 0 && (
              <button
                disabled={task.busy}
                onClick={() => {
                  void task.run(async () =>
                    setChanges(await api().PreviewReplacement(novelId)),
                  );
                }}
              >
                Xem trước thay tên toàn truyện
              </button>
            )}
          </div>
          {terms.length === 0 ? (
            <Empty
              icon={<BookA size={32} />}
              title="Mỗi cái tên, một cách viết"
            >
              <p>
                Thêm tên chuẩn và biến thể sai để giữ nhất quán trong mọi
                chương.
              </p>
            </Empty>
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>TÊN TRONG BẢN CONVERT</th>
                    <th>TÊN CHUẨN</th>
                    <th>PHẠM VI / LOẠI</th>
                    <th>ƯU TIÊN</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {terms
                    .filter((t) =>
                      `${t.source} ${t.target}`
                        .toLocaleLowerCase("vi")
                        .includes(search.toLocaleLowerCase("vi")),
                    )
                    .map((t) => (
                      <tr key={t.id}>
                        <td>
                          <strong>{t.source}</strong>
                          {t.variants.length > 0 && (
                            <small className="hint">
                              Biến thể: {t.variants.join(", ")}
                            </small>
                          )}
                        </td>
                        <td className="term-target">
                          {t.target} {t.locked && <Lock size={13} />}
                        </td>
                        <td>
                          <small>
                            {t.novelId ? "Riêng truyện" : "Global"} · {t.kind}
                          </small>
                        </td>
                        <td>{t.priority}</td>
                        <td>
                          <div className="actions">
                            <button
                              className="icon"
                              aria-label={`Sửa ${t.source}`}
                              onClick={() => setTerm(t)}
                            >
                              <Edit3 size={15} />
                            </button>
                            <button
                              className="icon"
                              disabled={task.busy || t.locked}
                              aria-label={`Xóa ${t.source}`}
                              onClick={() => {
                                void task.run(async () => {
                                  await api().DeleteTerm(t.id);
                                  await reload();
                                });
                              }}
                            >
                              <Trash2 size={15} />
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      ) : (
        <div className="panel form">
          {tab === "suggest" ? (
            <>
              <h2>Những cái tên xuất hiện nhiều lần</h2>
              <p className="hint">
                Quét tối đa 100 chương đã tải. Mỗi gợi ý cần được bạn duyệt
                trước khi sử dụng.
              </p>
              <button
                disabled={task.busy}
                onClick={() => {
                  void task.run(
                    async () => setSuggestions(await api().Suggest(novelId)),
                    "Đã quét tên. Danh sách chỉ gồm cụm tên lặp từ 2 lần.",
                  );
                }}
              >
                <Sparkles size={16} />
                Quét bản convert
              </button>
            </>
          ) : (
            <>
              <p>
                Dán các đoạn tương ứng của bản convert và bản dịch tham chiếu mà
                bạn được phép sử dụng.
              </p>
              <div className="form-grid">
                <label>
                  Bản convert
                  <textarea
                    rows={8}
                    value={raw}
                    onChange={(e) => setRaw(e.target.value)}
                  />
                </label>
                <label>
                  Bản tham chiếu
                  <textarea
                    rows={8}
                    value={reference}
                    onChange={(e) => setReference(e.target.value)}
                  />
                </label>
              </div>
              {raw &&
                reference &&
                raw.split(/\n\s*\n/).length !==
                  reference.split(/\n\s*\n/).length && (
                  <Notice message="Hai bản lệch số đoạn. Kiểm tra cặp đoạn trước khi duyệt làm ví dụ." />
                )}
              <div className="actions">
                <button
                  disabled={task.busy || !raw || !reference}
                  onClick={() => {
                    void task.run(
                      async () =>
                        setSuggestions(
                          await api().Learn(novelId, raw, reference),
                        ),
                      "Đã trích xuất. Hãy kiểm tra từng cặp tên.",
                    );
                  }}
                >
                  <Sparkles size={16} />
                  Trích cặp tên bằng model
                </button>
                <button
                  disabled={task.busy || !raw || !reference}
                  onClick={() => {
                    void task.run(async () => {
                      const style = await api().Style(novelId);
                      if (style.fewShot.length >= 3)
                        throw new Error(
                          "Đã đủ 3 ví dụ. Hãy thay thế bộ ví dụ trước khi thêm.",
                        );
                      await api().SaveStyle(novelId, {
                        ...style,
                        fewShot: [
                          ...style.fewShot,
                          { convert: raw, reference },
                        ],
                      });
                    }, "Đã duyệt cặp đoạn làm ví dụ cho prompt.");
                  }}
                >
                  Duyệt làm ví dụ few-shot
                </button>
                <button
                  disabled={task.busy}
                  onClick={() => {
                    void task.run(async () => {
                      const style = await api().Style(novelId);
                      await api().SaveStyle(novelId, { ...style, fewShot: [] });
                    }, "Đã xóa bộ ví dụ của truyện.");
                  }}
                >
                  Xóa bộ ví dụ
                </button>
              </div>
            </>
          )}
          {suggestions.length > 0 && (
            <table>
              <thead>
                <tr>
                  <th>TÊN NGUỒN</th>
                  <th>TÊN CHUẨN</th>
                  <th>SỐ LẦN</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {suggestions.map((p, i) => (
                  <tr key={`${p.source}:${i}`}>
                    <td>{p.source}</td>
                    <td>
                      <input
                        value={p.target}
                        onChange={(e) =>
                          setSuggestions((list) =>
                            list.map((v, at) =>
                              at === i ? { ...v, target: e.target.value } : v,
                            ),
                          )
                        }
                      />
                    </td>
                    <td>{p.count}</td>
                    <td>
                      <button
                        disabled={task.busy}
                        onClick={() => {
                          void approve(p);
                        }}
                      >
                        <Check size={15} />
                        Duyệt
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
      {term && (
        <Modal
          title={term.id ? "Sửa thuật ngữ" : "Thêm thuật ngữ"}
          close={() => setTerm(null)}
        >
          <div className="form">
            <div className="form-grid">
              <label>
                Tên trong bản convert
                <input
                  value={term.source}
                  onChange={(e) => setTerm({ ...term, source: e.target.value })}
                />
              </label>
              <label>
                Tên chuẩn
                <input
                  value={term.target}
                  onChange={(e) => setTerm({ ...term, target: e.target.value })}
                />
              </label>
              <label>
                Loại
                <select
                  value={term.kind}
                  onChange={(e) => setTerm({ ...term, kind: e.target.value })}
                >
                  <option value="name">Nhân vật</option>
                  <option value="place">Địa danh</option>
                  <option value="skill">Công pháp</option>
                  <option value="realm">Cảnh giới</option>
                  <option value="other">Khác</option>
                </select>
              </label>
              <label>
                Ưu tiên
                <input
                  type="number"
                  value={term.priority}
                  onChange={(e) =>
                    setTerm({ ...term, priority: Number(e.target.value) })
                  }
                />
              </label>
            </div>
            <label>
              Biến thể sai (mỗi dòng một tên)
              <textarea
                rows={3}
                value={term.variants.join("\n")}
                onChange={(e) =>
                  setTerm({ ...term, variants: e.target.value.split("\n") })
                }
              />
            </label>
            <label>
              Ghi chú
              <input
                value={term.note}
                onChange={(e) => setTerm({ ...term, note: e.target.value })}
              />
            </label>
            <label className="check">
              <input
                type="checkbox"
                checked={term.locked}
                onChange={(e) => setTerm({ ...term, locked: e.target.checked })}
              />
              Khóa để tránh bị import ghi đè
            </label>
            <label className="check">
              <input
                type="checkbox"
                checked={term.novelId === null}
                disabled={!novelId}
                onChange={(e) =>
                  setTerm({
                    ...term,
                    novelId: e.target.checked ? null : novelId,
                  })
                }
              />
              Áp dụng global
            </label>
            <Notice message={task.message} failed={task.failed} />
            <button
              className="primary"
              disabled={task.busy || !term.source.trim() || !term.target.trim()}
              onClick={() => {
                void task.run(async () => {
                  await api().SaveTerm(term, true);
                  await reload();
                  setTerm(null);
                }, "Đã lưu thuật ngữ. Các chunk tiếp theo sẽ dùng bảng mới.");
              }}
            >
              Lưu thuật ngữ
            </button>
          </div>
        </Modal>
      )}
      {changes && (
        <Modal
          title={`Xem trước: ${changes.length} chương thay đổi`}
          close={() => setChanges(null)}
        >
          <p>
            Các chương được thay sẽ được đánh dấu sửa tay. Có thể hoàn tác từng
            chương trong Reader.
          </p>
          <div className="replacement-preview">
            {changes.slice(0, 20).map((change) => (
              <details key={change.chapterId}>
                <summary>{change.title}</summary>
                <div className="form-grid">
                  <pre>{change.before}</pre>
                  <pre>{change.after}</pre>
                </div>
              </details>
            ))}
            {changes.length > 20 && (
              <p>Và {changes.length - 20} chương khác.</p>
            )}
          </div>
          <Notice message={task.message} failed={task.failed} />
          <button
            className="primary"
            disabled={task.busy || changes.length === 0}
            onClick={() => {
              void task.run(async () => {
                await api().ApplyReplacement(changes);
                setChanges(null);
              }, "Đã thay tên và lưu lịch sử hoàn tác.");
            }}
          >
            Áp dụng {changes.length} chương
          </button>
        </Modal>
      )}
    </>
  );
}
