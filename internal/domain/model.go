package domain

type Status string

const (
	Pending     Status = "pending"
	Queued      Status = "queued"
	Processing  Status = "processing"
	Done        Status = "done"
	NeedsReview Status = "needs_review"
	Running     Status = "running"
	Paused      Status = "paused"
	Interrupted Status = "interrupted"
	Cancelled   Status = "cancelled"
	Error       Status = "error"
)

type Novel struct {
	ID              int64  `json:"id"`
	Title           string `json:"title"`
	Author          string `json:"author"`
	Description     string `json:"description"`
	SourceURL       string `json:"sourceUrl"`
	LastReadChapter int64  `json:"lastReadChapter"`
	LastReadOffset  int    `json:"lastReadOffset"`
	ChapterCount    int    `json:"chapterCount"`
	DoneCount       int    `json:"doneCount"`
	ReviewCount     int    `json:"reviewCount"`
}
type Chapter struct {
	ID         int64  `json:"id"`
	NovelID    int64  `json:"novelId"`
	Index      int    `json:"index"`
	Title      string `json:"title"`
	SourceURL  string `json:"sourceUrl"`
	RawText    string `json:"rawText"`
	EditedText string `json:"editedText"`
	Summary    string `json:"summary"`
	Status     Status `json:"status"`
	ModelUsed  string `json:"modelUsed"`
	Flags      string `json:"flags"`
	ManualEdit bool   `json:"manualEdit"`
}
type Chunk struct {
	ID         int64  `json:"id"`
	ChapterID  int64  `json:"chapterId"`
	Index      int    `json:"index"`
	RawText    string `json:"rawText"`
	EditedText string `json:"editedText"`
	Status     Status `json:"status"`
	Attempts   int    `json:"attempts"`
	CrashCount int    `json:"crashCount"`
	Flags      string `json:"flags"`
}
type Job struct {
	ID        int64  `json:"id"`
	NovelID   int64  `json:"novelId"`
	Title     string `json:"title"`
	From      int    `json:"from"`
	To        int    `json:"to"`
	Status    Status `json:"status"`
	Priority  int    `json:"priority"`
	Error     string `json:"error"`
	AutoRetry bool   `json:"autoRetry"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
}
type Term struct {
	ID       int64    `json:"id"`
	NovelID  *int64   `json:"novelId"`
	Source   string   `json:"source"`
	Target   string   `json:"target"`
	Kind     string   `json:"kind"`
	Note     string   `json:"note"`
	Priority int      `json:"priority"`
	Locked   bool     `json:"locked"`
	Origin   string   `json:"origin"`
	Variants []string `json:"variants"`
}
type Pair struct {
	Convert   string `json:"convert"`
	Reference string `json:"reference"`
}
type Style struct {
	Pronouns string `json:"pronouns"`
	Tone     string `json:"tone"`
	FewShot  []Pair `json:"fewShot"`
}
type Config struct {
	OllamaURL     string  `json:"ollamaUrl"`
	Editor        string  `json:"editor"`
	Extractor     string  `json:"extractor"`
	Summarizer    string  `json:"summarizer"`
	Temperature   float64 `json:"temperature"`
	TopP          float64 `json:"topP"`
	RepeatPenalty float64 `json:"repeatPenalty"`
	NumCtx        int     `json:"numCtx"`
	NumPredict    int     `json:"numPredict"`
	KeepAlive     string  `json:"keepAlive"`
	ThinkOff      bool    `json:"thinkOff"`
	ChunkWords    int     `json:"chunkWords"`
	Retries       int     `json:"retries"`
	AutoResume    bool    `json:"autoResume"`
	NightOnly     bool    `json:"nightOnly"`
	NightStart    int     `json:"nightStart"`
	NightEnd      int     `json:"nightEnd"`
	Autostart     bool    `json:"autostart"`
	Notifications bool    `json:"notifications"`
	Prefetch      int     `json:"prefetch"`
}

func DefaultConfig() Config {
	return Config{OllamaURL: "http://127.0.0.1:11434", Temperature: 0.3, TopP: 0.9, RepeatPenalty: 1.1, NumCtx: 8192, NumPredict: 4096, KeepAlive: "10m", ThinkOff: true, ChunkWords: 700, Retries: 2, AutoResume: true, NightStart: 22, NightEnd: 7}
}
