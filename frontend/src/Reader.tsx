import { useEffect, useRef, useState } from "react";
import {
  ArrowLeft,
  ArrowRight,
  BookA,
  Columns2,
  Edit3,
  Save,
  Undo2,
} from "lucide-react";
import { api } from "./api";
import type { Chapter, Chunk, Novel, Route } from "./types";
import { Badge, Loading, Notice, useTask } from "./ui";

type Mode = "edited" | "raw" | "parallel";
type Theme = "light" | "sepia" | "dark";
export function Reader({
  novel,
  novelId,
  chapterId,
  navigate,
  refresh,
}: {
  novel: Novel | undefined;
  novelId: number;
  chapterId: number;
  navigate: (route: Route) => void;
  refresh: () => Promise<void>;
}) {
  const [chapter, setChapter] = useState<Chapter | null>(null);
  const [list, setList] = useState<Chapter[]>([]);
  const [chunks, setChunks] = useState<Chunk[]>([]);
  const [mode, setMode] = useState<Mode>("edited");
  const [theme, setTheme] = useState<Theme>("sepia");
  const [size, setSize] = useState(21);
  const [lineHeight, setLineHeight] = useState(1.9);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [selection, setSelection] = useState("");
  const task = useTask();
  const scroll = useRef<HTMLDivElement>(null);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const offset = useRef(0);
  const initialOffset = useRef(
    novel?.lastReadChapter === chapterId ? novel.lastReadOffset : 0,
  );
  useEffect(() => {
    let alive = true;
    void task.run(async () => {
      const [ch, chapters, parts] = await Promise.all([
        api().Chapter(chapterId),
        api().Chapters(novelId),
        api().Chunks(chapterId),
      ]);
      if (alive) {
        setChapter(ch);
        setList(chapters);
        setChunks(parts);
        setDraft(ch.editedText || ch.rawText);
      }
    });
    return () => {
      alive = false;
      if (saveTimer.current) clearTimeout(saveTimer.current);
    };
  }, [chapterId, novelId, task.run]);
  useEffect(() => {
    if (chapter && scroll.current) {
      scroll.current.scrollTop = initialOffset.current;
      offset.current = initialOffset.current;
    }
  }, [chapter]);
  useEffect(() => {
    try {
      const saved = localStorage.getItem("reader-theme");
      if (saved === "light" || saved === "dark" || saved === "sepia")
        setTheme(saved);
      const n = Number(localStorage.getItem("reader-size"));
      if (n >= 16 && n <= 32) setSize(n);
    } catch {
      /* Preferences remain available for this session. */
    }
  }, []);
  const dirty =
    editing && draft !== (chapter?.editedText || chapter?.rawText || "");
  useEffect(() => {
    const guard = (e: BeforeUnloadEvent) => {
      if (dirty) e.preventDefault();
    };
    window.addEventListener("beforeunload", guard);
    const navGuard = (e: Event) => {
      if (
        dirty &&
        !window.confirm("Bản sửa chưa lưu. Rời trang và bỏ thay đổi?")
      )
        e.preventDefault();
    };
    window.addEventListener("convertfix:navigate", navGuard);
    return () => {
      window.removeEventListener("beforeunload", guard);
      window.removeEventListener("convertfix:navigate", navGuard);
    };
  }, [dirty]);
  const move = (route: Route) => {
    if (saveTimer.current) clearTimeout(saveTimer.current);
    void api()
      .SavePosition(novelId, chapterId, offset.current)
      .catch(() => undefined);
    navigate(route);
  };
  const changeTheme = (value: Theme) => {
    setTheme(value);
    try {
      localStorage.setItem("reader-theme", value);
    } catch {
      /* Session preference still applies. */
    }
  };
  const index = list.findIndex((c) => c.id === chapterId);
  const previous = list[index - 1];
  const next = list[index + 1];
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (
        e.target instanceof HTMLElement &&
        (e.target.matches("input,textarea,select") ||
          e.target.isContentEditable)
      )
        return;
      const target =
        e.key === "ArrowLeft"
          ? previous
          : e.key === "ArrowRight"
            ? next
            : undefined;
      if (target) {
        e.preventDefault();
        navigate({ kind: "reader", novelId, chapterId: target.id });
      }
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  }, [previous, next, novelId, navigate]);
  const paragraph = (text: string) =>
    text
      .split(/\n\s*\n/)
      .filter(Boolean)
      .map((p, i) => <p key={i}>{p}</p>);
  return (
    <div className={`reader theme-${theme}`}>
      <div className="reader-toolbar">
        <button
          className="icon"
          title="Về danh sách chương"
          onClick={() => move({ kind: "novel", novelId })}
        >
          <ArrowLeft size={19} />
        </button>
        <strong>{novel?.title}</strong>
        <div className="reader-controls">
          <select
            value={mode}
            onChange={(e) => {
              const value = e.target.value;
              if (value === "edited" || value === "raw" || value === "parallel")
                setMode(value);
            }}
            aria-label="Chế độ đọc"
          >
            <option value="edited">Bản biên tập</option>
            <option value="raw">Bản convert</option>
            <option value="parallel">Song song</option>
          </select>
          <select
            value={theme}
            onChange={(e) => {
              const value = e.target.value;
              if (value === "light" || value === "sepia" || value === "dark")
                changeTheme(value);
            }}
            aria-label="Màu nền"
          >
            <option value="sepia">Giấy ngà</option>
            <option value="light">Sáng</option>
            <option value="dark">Tối</option>
          </select>
          <label className="range-label">
            Aa
            <input
              type="range"
              min={16}
              max={32}
              value={size}
              onChange={(e) => {
                const n = Number(e.target.value);
                setSize(n);
                try {
                  localStorage.setItem("reader-size", String(n));
                } catch {
                  /* Session preference still applies. */
                }
              }}
            />
          </label>
          <label className="range-label">
            ↕
            <input
              type="range"
              min={1.4}
              max={2.4}
              step={0.1}
              value={lineHeight}
              onChange={(e) => setLineHeight(Number(e.target.value))}
            />
          </label>
        </div>
      </div>
      <Notice message={task.message} failed={task.failed} />
      {!chapter ? (
        <Loading />
      ) : (
        <>
          <div
            className="reader-scroll"
            ref={scroll}
            onMouseUp={() => {
              const selected = window.getSelection()?.toString().trim() ?? "";
              setSelection(selected.length <= 120 ? selected : "");
            }}
            onScroll={(e) => {
              offset.current = Math.round(e.currentTarget.scrollTop);
              if (saveTimer.current) clearTimeout(saveTimer.current);
              saveTimer.current = setTimeout(() => {
                void api()
                  .SavePosition(novelId, chapterId, offset.current)
                  .catch((error) => task.setMessage(String(error)));
              }, 1200);
            }}
          >
            <div
              className={`reader-content ${mode === "parallel" ? "wide" : ""}`}
              style={{
                fontSize: size,
                lineHeight,
              }}
            >
              <div className="chapter-heading">
                <span className="eyebrow">
                  {chapter.flags.includes("catalog_position=")
                    ? "SỐ THỨ TỰ"
                    : "CHƯƠNG"}{" "}
                  {chapter.index}
                </span>
                <h1>{chapter.title}</h1>
                <Badge status={chapter.status} />
                {chapter.manualEdit && (
                  <span className="badge">Đã sửa tay</span>
                )}
              </div>
              {chapter.flags && (
                <Notice message={`Cần kiểm tra: ${chapter.flags}`} />
              )}
              {!chapter.editedText && (
                <Notice message="Chưa có bản biên tập. Đang hiển thị bản convert." />
              )}
              {!chapter.rawText && (
                <Notice message="Chương chưa tải. Hãy tạo job từ trang chi tiết truyện." />
              )}
              <div className="reader-edit-actions">
                <button
                  onClick={() => {
                    if (dirty && !window.confirm("Bỏ thay đổi chưa lưu?"))
                      return;
                    setDraft(chapter.editedText || chapter.rawText);
                    setEditing(!editing);
                  }}
                >
                  <Edit3 size={15} />
                  {editing ? "Đóng chỉnh sửa" : "Sửa chương"}
                </button>
                <button
                  disabled={task.busy}
                  onClick={() => {
                    void task.run(async () => {
                      await api().UndoEdit(chapterId);
                      const updated = await api().Chapter(chapterId);
                      setChapter(updated);
                      setDraft(updated.editedText || updated.rawText);
                      await refresh();
                    }, "Đã hoàn tác bản sửa gần nhất.");
                  }}
                >
                  <Undo2 size={15} />
                  Hoàn tác
                </button>
                {selection && (
                  <button
                    onClick={() =>
                      move({ kind: "glossary", novelId, selection })
                    }
                  >
                    <BookA size={15} />
                    Thêm “{selection.slice(0, 30)}”
                  </button>
                )}
              </div>
              {editing ? (
                <div className="manual-editor">
                  <textarea
                    value={draft}
                    onChange={(e) => setDraft(e.target.value)}
                    rows={24}
                    style={{ fontSize: size, lineHeight }}
                  />
                  <button
                    className="primary"
                    disabled={task.busy || !draft.trim()}
                    onClick={() => {
                      void task.run(async () => {
                        await api().SaveManual(chapterId, draft);
                        const updated = await api().Chapter(chapterId);
                        setChapter(updated);
                        setDraft(updated.editedText);
                        setEditing(false);
                        await refresh();
                      }, "Đã lưu. Bản sửa được bảo vệ khỏi biên tập tự động.");
                    }}
                  >
                    <Save size={16} />
                    Lưu bản sửa
                  </button>
                </div>
              ) : mode === "parallel" ? (
                <>
                  <div className="parallel-head">
                    <span>
                      <Columns2 size={16} />
                      BẢN CONVERT
                    </span>
                    <span>BẢN BIÊN TẬP</span>
                  </div>
                  {chunks.length > 0 && !chapter.manualEdit ? (
                    chunks.map((chunk) => (
                      <div className="parallel-row" key={chunk.id}>
                        <div>{paragraph(chunk.rawText)}</div>
                        <div>
                          {chunk.status === "needs_review" && (
                            <Badge status="needs_review" />
                          )}
                          {paragraph(chunk.editedText || chunk.rawText)}
                        </div>
                      </div>
                    ))
                  ) : (
                    <div className="parallel-row">
                      <div>{paragraph(chapter.rawText)}</div>
                      <div>
                        {paragraph(chapter.editedText || chapter.rawText)}
                      </div>
                    </div>
                  )}
                </>
              ) : (
                <article>
                  {paragraph(
                    mode === "raw"
                      ? chapter.rawText
                      : chapter.editedText || chapter.rawText,
                  )}
                </article>
              )}
              <div className="reader-end">◆</div>
            </div>
          </div>
          <div className="reader-bottom">
            <button
              disabled={!previous}
              onClick={() => {
                if (previous)
                  move({ kind: "reader", novelId, chapterId: previous.id });
              }}
            >
              <ArrowLeft size={16} />
              Chương trước
            </button>
            <span>
              {index + 1} / {list.length} · Dùng phím ← → để chuyển chương
            </span>
            <button
              disabled={!next}
              onClick={() => {
                if (next) move({ kind: "reader", novelId, chapterId: next.id });
              }}
            >
              Chương sau
              <ArrowRight size={16} />
            </button>
          </div>
        </>
      )}
    </div>
  );
}
