import { useCallback, useEffect, useState } from "react";
import {
  BookOpen,
  Library,
  ListOrdered,
  Plus,
  Settings2,
  BookA,
  ArrowUpRight,
  HardDrive,
  Search,
  ChevronRight,
  Sparkles,
} from "lucide-react";
import { api } from "./api";
import type { Route, State } from "./types";
import { Empty, Heading, Notice } from "./ui";
import { AddPage, NovelPage, QueuePage } from "./LibraryPages";
import { Reader } from "./Reader";
import { GlossaryPage } from "./GlossaryPage";
import { SettingsPage } from "./SettingsPage";

export function App() {
  const [route, updateRoute] = useState<Route>({ kind: "library" });
  const setRoute = useCallback((next: Route) => {
    if (
      window.dispatchEvent(
        new Event("convertfix:navigate", { cancelable: true }),
      )
    )
      updateRoute(next);
  }, []);
  const [state, setState] = useState<State>({
    novels: [],
    jobs: [],
    progress: {
      jobId: 0,
      chapter: 0,
      chunk: 0,
      phase: "",
      tokensPerSecond: 0,
      etaSeconds: 0,
      message: "",
    },
    dataDir: "",
  });
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const refresh = useCallback(async () => {
    const result = await api().State();
    setState(result);
    setError("");
  }, []);
  useEffect(() => {
    let alive = true;
    let pending = false;
    const poll = async () => {
      if (pending) return;
      pending = true;
      try {
        const next = await api().State();
        if (alive) {
          setState(next);
          setError("");
        }
      } catch (e) {
        if (alive) setError(e instanceof Error ? e.message : String(e));
      } finally {
        pending = false;
      }
    };
    void poll();
    const timer = setInterval(() => {
      void poll();
    }, 2500);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, []);
  const running = state.jobs.filter((j) => j.status === "running").length;
  const active = state.jobs.filter(
    (j) => !["done", "cancelled", "error"].includes(j.status),
  ).length;
  const total = state.novels.reduce((sum, n) => sum + n.chapterCount, 0);
  const done = state.novels.reduce((sum, n) => sum + n.doneCount, 0);
  const selected =
    "novelId" in route
      ? state.novels.find((n) => n.id === route.novelId)
      : undefined;
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <a
          className="brand"
          href="#"
          onClick={(e) => {
            e.preventDefault();
            setRoute({ kind: "library" });
          }}
        >
          <div className="brand-mark">
            <BookOpen size={23} />
          </div>
          <span>
            Convert<span className="brand-accent">Fix</span>
            <small>GÓC ĐỌC CỦA BẠN</small>
          </span>
        </a>
        <div className="nav-label">KHÔNG GIAN LÀM VIỆC</div>
        <nav>
          <button
            className={
              route.kind === "library" ||
              route.kind === "novel" ||
              route.kind === "reader"
                ? "selected"
                : ""
            }
            onClick={() => setRoute({ kind: "library" })}
          >
            <Library size={19} />
            Thư viện
          </button>
          <button
            className={route.kind === "add" ? "selected" : ""}
            onClick={() => setRoute({ kind: "add" })}
          >
            <Plus size={19} />
            Thêm truyện
          </button>
          <button
            className={route.kind === "queue" ? "selected" : ""}
            onClick={() => setRoute({ kind: "queue" })}
          >
            <ListOrdered size={19} />
            Hàng đợi {active > 0 && <span className="nav-count">{active}</span>}
          </button>
          <button
            className={route.kind === "glossary" ? "selected" : ""}
            onClick={() => setRoute({ kind: "glossary", novelId: 0 })}
          >
            <BookA size={19} />
            Thuật ngữ
          </button>
          <button
            className={route.kind === "settings" ? "selected" : ""}
            onClick={() => setRoute({ kind: "settings" })}
          >
            <Settings2 size={19} />
            Cài đặt
          </button>
        </nav>
        <div className="sidebar-note">
          <Sparkles size={19} />
          <strong>Giữ trọn câu chuyện.</strong>
          <p>
            Biên tập tự nhiên.
            <br />
            Nhất quán từng cái tên.
          </p>
        </div>
        <div className="local-badge">
          <span className="status-dot" /> Dữ liệu lưu trên máy bạn
          <small>ConvertFix · 0.1.0</small>
        </div>
      </aside>
      <div className="workspace">
        <div className="topbar">
          <span>
            Không gian cá nhân <ChevronRight size={14} />{" "}
            {selected?.title ?? "ConvertFix"}
          </span>
          <span className="top-status">
            <span className={`status-dot ${running ? "" : "muted"}`} />
            {running ? "Đang biên tập" : "Sẵn sàng"}
          </span>
        </div>
        <main className={route.kind === "reader" ? "reader-main" : ""}>
          <Notice message={error} failed />
          {route.kind === "library" && (
            <>
              <Heading
                eyebrow="THƯ VIỆN CÁ NHÂN"
                title="Một câu chuyện, trọn vẹn hơn."
                actions={
                  <button
                    className="primary"
                    onClick={() => setRoute({ kind: "add" })}
                  >
                    <Plus size={18} />
                    Thêm truyện
                  </button>
                }
              >
                Từ bản convert đến những trang truyện dễ đọc, theo cách của bạn.
              </Heading>
              <div className="stats">
                <div>
                  <span>TRUYỆN TRONG THƯ VIỆN</span>
                  <strong>
                    {state.novels.length.toString().padStart(2, "0")}
                  </strong>
                  <Library size={22} />
                </div>
                <div>
                  <span>CHƯƠNG ĐÃ BIÊN TẬP</span>
                  <strong>
                    {done.toLocaleString("vi-VN")}
                    <small> / {total.toLocaleString("vi-VN")}</small>
                  </strong>
                  <BookOpen size={22} />
                </div>
                <div>
                  <span>HÀNG ĐỢI HIỆN TẠI</span>
                  <strong>
                    {active.toString().padStart(2, "0")}
                    <small> công việc</small>
                  </strong>
                  <ListOrdered size={22} />
                </div>
              </div>
              {active > 0 && (
                <button
                  className="queue-banner"
                  onClick={() => setRoute({ kind: "queue" })}
                >
                  <div>
                    <span className="status-dot" />
                    <strong>
                      {running
                        ? "Câu chuyện của bạn đang được biên tập"
                        : "Hàng đợi đang chờ"}
                    </strong>
                    <small>
                      {running
                        ? `Chương ${state.progress.chapter} · chunk ${state.progress.chunk}`
                        : "Xem tiến độ và tiếp tục công việc"}
                    </small>
                  </div>
                  <ArrowUpRight size={20} />
                </button>
              )}
              <div className="section-heading">
                <h2>
                  Tất cả truyện <span>{state.novels.length}</span>
                </h2>
                <label className="search">
                  <Search size={17} />
                  <input
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    placeholder="Tìm trong thư viện…"
                    aria-label="Tìm truyện"
                  />
                </label>
              </div>
              {state.novels.length === 0 ? (
                <Empty
                  icon={<BookOpen size={36} />}
                  title="Trang đầu tiên đang chờ bạn"
                >
                  <p>
                    Thêm nguồn truyện hoặc nhập bản convert.
                    <br />
                    ConvertFix sẽ giúp bạn biên tập từng chương.
                  </p>
                  <button
                    className="primary"
                    onClick={() => setRoute({ kind: "add" })}
                  >
                    <Plus size={17} />
                    Thêm truyện đầu tiên
                  </button>
                </Empty>
              ) : (
                <div className="book-grid">
                  {state.novels
                    .filter((n) =>
                      n.title
                        .toLocaleLowerCase("vi")
                        .includes(search.toLocaleLowerCase("vi")),
                    )
                    .map((n, i) => (
                      <button
                        className="book-card"
                        key={n.id}
                        onClick={() =>
                          setRoute({ kind: "novel", novelId: n.id })
                        }
                      >
                        <div className={`book-cover cover-${i % 4}`}>
                          <div className="cover-line" />
                          <span>CONVERTFIX LIBRARY</span>
                          <h3>{n.title}</h3>
                          <BookOpen size={28} />
                          <small>{n.author || "Truyện của bạn"}</small>
                        </div>
                        <div className="book-info">
                          <h3>{n.title}</h3>
                          <p>
                            {n.chapterCount} chương ·{" "}
                            {n.reviewCount
                              ? `${n.reviewCount} cần xem lại`
                              : "Lưu trên máy"}
                          </p>
                          <progress
                            max={Math.max(1, n.chapterCount)}
                            value={n.doneCount}
                          />
                          <div>
                            <span>{n.doneCount} chương đã biên tập</span>
                            <ChevronRight size={16} />
                          </div>
                        </div>
                      </button>
                    ))}
                </div>
              )}
              <footer className="library-footer">
                <HardDrive size={15} />
                Bản gốc luôn được giữ lại để đối chiếu.
              </footer>
            </>
          )}
          {route.kind === "add" && (
            <AddPage navigate={setRoute} refresh={refresh} />
          )}
          {route.kind === "novel" && (
            <NovelPage
              key={route.novelId}
              novel={selected}
              novelId={route.novelId}
              navigate={setRoute}
              refresh={refresh}
            />
          )}
          {route.kind === "reader" && (
            <Reader
              key={route.chapterId}
              novel={selected}
              novelId={route.novelId}
              chapterId={route.chapterId}
              navigate={setRoute}
              refresh={refresh}
            />
          )}
          {route.kind === "queue" && (
            <QueuePage state={state} refresh={refresh} />
          )}
          {route.kind === "glossary" && (
            <GlossaryPage
              key={`${route.novelId}:${route.selection ?? ""}`}
              novelId={route.novelId}
              selection={route.selection ?? ""}
              novels={state.novels}
              navigate={setRoute}
            />
          )}
          {route.kind === "settings" && <SettingsPage />}
        </main>
      </div>
    </div>
  );
}
