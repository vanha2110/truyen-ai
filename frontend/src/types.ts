export type ChapterStatus =
  | "pending"
  | "queued"
  | "processing"
  | "done"
  | "needs_review"
  | "error";
export type JobStatus =
  | "queued"
  | "running"
  | "paused"
  | "interrupted"
  | "done"
  | "cancelled"
  | "error";
export interface Novel {
  id: number;
  title: string;
  author: string;
  description: string;
  sourceUrl: string;
  lastReadChapter: number;
  lastReadOffset: number;
  chapterCount: number;
  doneCount: number;
  reviewCount: number;
}
export interface Chapter {
  id: number;
  novelId: number;
  index: number;
  title: string;
  sourceUrl: string;
  rawText: string;
  editedText: string;
  summary: string;
  status: ChapterStatus;
  modelUsed: string;
  flags: string;
  manualEdit: boolean;
}
export interface Chunk {
  id: number;
  chapterId: number;
  index: number;
  rawText: string;
  editedText: string;
  status: "pending" | "processing" | "done" | "needs_review";
  attempts: number;
  crashCount: number;
  flags: string;
}
export interface Job {
  id: number;
  novelId: number;
  title: string;
  from: number;
  to: number;
  status: JobStatus;
  priority: number;
  error: string;
  autoRetry: boolean;
  completed: number;
  total: number;
}
export interface Progress {
  jobId: number;
  chapter: number;
  chunk: number;
  phase: string;
  tokensPerSecond: number;
  etaSeconds: number;
  message: string;
}
export interface State {
  novels: Novel[];
  jobs: Job[];
  progress: Progress;
  dataDir: string;
}
export interface Term {
  id: number;
  novelId: number | null;
  source: string;
  target: string;
  kind: string;
  note: string;
  priority: number;
  locked: boolean;
  origin: string;
  variants: string[];
}
export interface Pair {
  convert: string;
  reference: string;
}
export interface Style {
  pronouns: string;
  tone: string;
  fewShot: Pair[];
}
export interface Source {
  title: string;
  author: string;
  description: string;
  chapterLinks: string;
  content: string;
  next: string;
  remove: string;
}
export interface Preview {
  novel: Novel;
  chapters: Chapter[];
  warnings: string[];
  pages: number;
  numbering: "source" | "catalog";
}
export interface Config {
  ollamaUrl: string;
  editor: string;
  extractor: string;
  summarizer: string;
  temperature: number;
  topP: number;
  repeatPenalty: number;
  numCtx: number;
  numPredict: number;
  keepAlive: string;
  thinkOff: boolean;
  chunkWords: number;
  retries: number;
  autoResume: boolean;
  nightOnly: boolean;
  nightStart: number;
  nightEnd: number;
  autostart: boolean;
  notifications: boolean;
  prefetch: number;
}
export interface CatalogModel {
  name: string;
  size: number;
  family: string;
  parameters: string;
  quantization: string;
  context: number;
  completion: boolean;
  loaded: boolean;
  gpuRatio: number;
  error: string;
}
export interface Catalog {
  version: string;
  models: CatalogModel[];
}
export interface Preset {
  name: string;
  config: Config;
}
export interface Benchmark {
  model: string;
  tokensPerSecond: number;
  loadMs: number;
  gpuRatio: number;
  seconds: number;
  result: { text: string; flags: string[]; ratio: number };
  estimated50: number;
  estimated100: number;
}
export interface Suggestion {
  source: string;
  target: string;
  count: number;
}
export interface Replacement {
  chapterId: number;
  title: string;
  before: string;
  after: string;
}
export interface UpdateChannel {
  url: string;
  publicKey: string;
}
export interface Release {
  version: string;
  url: string;
  sha256: string;
  signature: string;
  notes: string;
}
export type Route =
  | { kind: "library" | "add" | "queue" | "settings" }
  | { kind: "novel"; novelId: number }
  | { kind: "reader"; novelId: number; chapterId: number }
  | { kind: "glossary"; novelId: number; selection?: string };
export const defaultSource: Source = {
  title: "h1",
  author: "[itemprop=author]",
  description: "[itemprop=description]",
  chapterLinks: "a[href]",
  content: "",
  next: "",
  remove:
    "script,style,nav,footer,header,aside,iframe,form,button,.ads,.advertisement",
};
export const newTerm = (novelId: number, source = ""): Term => ({
  id: 0,
  novelId: novelId || null,
  source,
  target: source,
  kind: "name",
  note: "",
  priority: 0,
  locked: false,
  origin: "manual",
  variants: [],
});
