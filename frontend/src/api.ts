import type {
  Benchmark,
  Catalog,
  Chapter,
  Chunk,
  Config,
  Pair,
  Preset,
  Preview,
  Release,
  Replacement,
  Source,
  State,
  Style,
  Suggestion,
  Term,
  UpdateChannel,
} from "./types";
interface Backend {
  State(): Promise<State>;
  Config(): Promise<Config>;
  SaveConfig(config: Config): Promise<void>;
  Chapters(novel: number): Promise<Chapter[]>;
  Chapter(id: number): Promise<Chapter>;
  Chunks(id: number): Promise<Chunk[]>;
  ImportText(title: string, text: string): Promise<number>;
  Preview(url: string, source: Source): Promise<Preview>;
  AddSource(preview: Preview, source: Source): Promise<number>;
  Source(novel: number): Promise<Source>;
  SaveSource(novel: number, source: Source): Promise<void>;
  AddJob(
    novel: number,
    from: number,
    to: number,
    priority: number,
  ): Promise<number>;
  ControlJob(id: number, action: "pause" | "resume" | "cancel"): Promise<void>;
  Priority(id: number, priority: number): Promise<void>;
  RetryChapter(
    id: number,
    onlyErrors: boolean,
    confirmManual: boolean,
  ): Promise<number>;
  SavePosition(novel: number, chapter: number, offset: number): Promise<void>;
  SaveManual(chapter: number, text: string): Promise<void>;
  UndoEdit(chapter: number): Promise<void>;
  Terms(novel: number): Promise<Term[]>;
  SaveTerm(term: Term, confirmLocked: boolean): Promise<void>;
  DeleteTerm(id: number): Promise<void>;
  ImportTerms(
    data: string,
    format: "json" | "csv",
    novel: number | null,
  ): Promise<void>;
  ExportTerms(novel: number, format: "json" | "csv"): Promise<string>;
  Style(novel: number): Promise<Style>;
  SaveStyle(novel: number, style: Style): Promise<void>;
  SaveOverride(novel: number, config: Config | null): Promise<void>;
  NovelConfig(novel: number): Promise<Config>;
  Catalog(host: string): Promise<Catalog>;
  PullModel(host: string, name: string): Promise<void>;
  CancelPull(): Promise<void>;
  Benchmark(config: Config, sample: string): Promise<Benchmark>;
  Presets(): Promise<Preset[]>;
  SavePreset(name: string, config: Config): Promise<void>;
  Backup(): Promise<string>;
  Suggest(novel: number): Promise<Suggestion[]>;
  Learn(
    novel: number,
    convert: string,
    reference: string,
  ): Promise<Suggestion[]>;
  PreviewReplacement(novel: number): Promise<Replacement[]>;
  ApplyReplacement(changes: Replacement[]): Promise<void>;
  ImportConfig(raw: string): Promise<void>;
  Diagnostics(): Promise<string>;
  UpdateChannel(): Promise<UpdateChannel>;
  SaveUpdateChannel(channel: UpdateChannel): Promise<void>;
  CheckUpdate(): Promise<Release>;
  DownloadUpdate(): Promise<string>;
}
declare global {
  interface Window {
    go?: { service: { App: Backend } };
  }
}
export function api(): Backend {
  const backend = window.go?.service.App;
  if (!backend)
    throw new Error(
      "Hãy mở ConvertFix.exe hoặc chạy wails dev để kết nối với core Go.",
    );
  return backend;
}
export function errorMessage(
  error: object | string | number | boolean | null | undefined,
): string {
  return error instanceof Error
    ? error.message
    : typeof error === "string"
      ? error
      : "Thao tác không thành công";
}
export function download(name: string, content: string) {
  const blob = new Blob([content], {
    type: name.endsWith(".csv")
      ? "text/csv;charset=utf-8"
      : "application/json;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}
export function pair(convert: string, reference: string): Pair {
  return { convert, reference };
}
