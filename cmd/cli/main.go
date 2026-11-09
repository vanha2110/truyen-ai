package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"convertfix/internal/crawler"
	"convertfix/internal/domain"
	"convertfix/internal/glossary"
	"convertfix/internal/ollama"
	"convertfix/internal/processor"
	"convertfix/internal/service"
	"convertfix/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func printJSON(v interface{}) error {
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
func run() error {
	if len(os.Args) < 2 {
		return errors.New("Lệnh: models | bench <model> [file] | edit <file> --model NAME | import <file> | add <url> | fetch --novel ID --from N --to N | process --novel ID --from N --to N --model NAME | run | jobs | backup")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	s, err := store.Open(store.DefaultDir())
	if err != nil {
		return err
	}
	defer s.Close()
	app, err := service.New(ctx, s, nil)
	if err != nil {
		return err
	}
	defer func() { cancel(); app.Queue.Wait() }()
	c, err := s.Config()
	if err != nil {
		return err
	}
	switch os.Args[1] {
	case "models":
		result, err := app.Catalog(c.OllamaURL)
		if err != nil {
			return err
		}
		return printJSON(result)
	case "bench":
		if len(os.Args) < 3 {
			return errors.New("bench <model> [file]")
		}
		c.Editor = os.Args[2]
		sample := "Lâm Phong đứng trước sơn môn, trong lòng có chút thấp thỏm. Hắn nhìn về phía lão giả đang lặng lẽ chờ đợi.\n\nLão giả vuốt râu, chậm rãi nói rằng muốn nhập môn thì trước hết phải vượt qua khảo nghiệm."
		if len(os.Args) > 3 {
			b, err := os.ReadFile(os.Args[3])
			if err != nil {
				return err
			}
			sample = string(b)
		}
		result, err := app.Benchmark(c, sample)
		if err != nil {
			return err
		}
		return printJSON(result)
	case "import":
		if len(os.Args) < 3 {
			return errors.New("import <file.txt>")
		}
		b, err := os.ReadFile(os.Args[2])
		if err != nil {
			return err
		}
		id, err := app.ImportText(strings.TrimSuffix(filepath.Base(os.Args[2]), filepath.Ext(os.Args[2])), string(b))
		if err != nil {
			return err
		}
		fmt.Println(id)
		return nil
	case "add":
		if len(os.Args) < 3 {
			return errors.New("add <url> [source.json]")
		}
		source := crawler.DefaultSource()
		if len(os.Args) > 3 {
			b, err := os.ReadFile(os.Args[3])
			if err != nil {
				return err
			}
			if err = json.Unmarshal(b, &source); err != nil {
				return err
			}
		}
		preview, err := app.Preview(os.Args[2], source)
		if err != nil {
			return err
		}
		id, err := app.AddSource(preview, source)
		if err != nil {
			return err
		}
		fmt.Println(id)
		return nil
	case "edit":
		if len(os.Args) < 3 {
			return errors.New("edit <file.txt> --model NAME")
		}
		flags := flag.NewFlagSet("edit", flag.ContinueOnError)
		model := flags.String("model", c.Editor, "Ollama model")
		if err = flags.Parse(os.Args[3:]); err != nil {
			return err
		}
		if *model == "" {
			return errors.New("Cần --model")
		}
		c.Editor = *model
		b, err := os.ReadFile(os.Args[2])
		if err != nil {
			return err
		}
		client, err := ollama.New(c.OllamaURL)
		if err != nil {
			return err
		}
		info, err := client.Show(ctx, *model)
		if err != nil {
			return err
		}
		previous := ""
		for _, part := range processor.Split(string(b), c.ChunkWords) {
			sys, user := processor.Prompt(part, processor.Tail(previous), nil, domain.Style{}, "")
			r, err := client.Chat(ctx, ollama.NewRequest(*model, sys, user, c, info), nil)
			if err != nil {
				return err
			}
			v := processor.Validate(part, r.Message.Content, glossary.New(nil))
			if len(v.Flags) > 0 {
				fmt.Fprintf(os.Stderr, "needs_review: %s\n", strings.Join(v.Flags, ","))
				v.Text = part
			}
			fmt.Println(v.Text + "\n")
			fmt.Fprintf(os.Stderr, "%.2f token/s; ratio %.2f\n", r.TokensPerSecond(), v.Ratio)
			previous = v.Text
		}
		return nil
	case "fetch", "process":
		flags := flag.NewFlagSet(os.Args[1], flag.ContinueOnError)
		novel := flags.Int64("novel", 0, "Novel ID")
		from := flags.Int("from", 1, "First chapter")
		to := flags.Int("to", 1, "Last chapter")
		model := flags.String("model", c.Editor, "Editor model")
		if err = flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		if *novel < 1 || *from < 1 || *to < *from {
			return errors.New("Khoảng chương không hợp lệ")
		}
		if os.Args[1] == "fetch" {
			source := crawler.DefaultSource()
			if err = s.GetJSON(fmt.Sprintf("source:%d", *novel), &source); err != nil {
				return err
			}
			cr := crawler.New()
			chapters, err := s.Chapters(*novel)
			if err != nil {
				return err
			}
			for _, entry := range chapters {
				if entry.Index < *from || entry.Index > *to {
					continue
				}
				ch, err := s.Chapter(entry.ID)
				if err != nil {
					return err
				}
				if ch.RawText != "" {
					continue
				}
				raw, err := cr.Chapter(ctx, ch.SourceURL, source)
				if err != nil {
					return err
				}
				if err = s.SaveRaw(ch.ID, raw); err != nil {
					return err
				}
				fmt.Printf("Downloaded chapter %d\n", ch.Index)
			}
			return nil
		}
		c.Editor = *model
		if err = app.SaveConfig(c); err != nil {
			return err
		}
		id, err := app.AddJob(*novel, *from, *to, 0)
		if err != nil {
			return err
		}
		if err = app.Queue.Start(ctx); err != nil {
			return err
		}
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				jobs, err := s.Jobs()
				if err != nil {
					return err
				}
				for _, j := range jobs {
					if j.ID == id {
						if j.Status == domain.Done {
							return printJSON(j)
						}
						if j.Status == domain.Paused && !j.AutoRetry {
							return errors.New(j.Error)
						}
						if j.Status == domain.Error || j.Status == domain.Cancelled {
							return fmt.Errorf("Job %s: %s", j.Status, j.Error)
						}
					}
				}
			}
		}
	case "run":
		if err = app.Queue.Start(ctx); err != nil {
			return err
		}
		<-ctx.Done()
		return nil
	case "jobs":
		jobs, err := s.Jobs()
		if err != nil {
			return err
		}
		return printJSON(jobs)
	case "backup":
		path, err := s.Backup()
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	default:
		return fmt.Errorf("Lệnh không xác định: %s", os.Args[1])
	}
}
