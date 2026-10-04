// Command spellicon generates one 128px WebP from a raw prompt.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/promix1722/easydnd/internal/usecase/spellicon"
)

type generator interface {
	Generate(context.Context, string) ([]byte, error)
}
type factory func(string, spellicon.Options) generator

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := run(ctx, os.Args[1:], os.Getenv("OPENAI_API_KEY"), os.Stderr,
		func(key string, opts spellicon.Options) generator { return spellicon.New(key, opts) })
	if err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "spellicon:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, key string, stderr io.Writer, newGenerator factory) error {
	fs := flag.NewFlagSet("spellicon", flag.ContinueOnError)
	fs.SetOutput(stderr)
	prompt := fs.String("prompt", "", "raw image prompt (required)")
	out := fs.String("out", "", "destination WebP file (required)")
	model := fs.String("model", spellicon.DefaultModel, "OpenAI image model")
	timeout := fs.Duration("timeout", spellicon.DefaultTimeout, "generation deadline including retries")
	overwrite := fs.Bool("overwrite", false, "replace an existing output file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*prompt) == "" || strings.TrimSpace(*out) == "" {
		return fmt.Errorf("-prompt and -out are required; positional arguments are unsupported")
	}
	if strings.TrimSpace(*model) == "" || *timeout <= 0 {
		return fmt.Errorf("model and positive timeout are required")
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("OPENAI_API_KEY is not set")
	}
	if info, err := os.Lstat(*out); err == nil {
		if !*overwrite || !info.Mode().IsRegular() {
			return fmt.Errorf("destination exists (use -overwrite for a regular file)")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	dir := filepath.Dir(*out)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	// Creating the temporary file before generation checks destination access.
	temp, err := os.CreateTemp(dir, ".spellicon-*.webp")
	if err != nil {
		return err
	}
	defer func() { _ = temp.Close(); _ = os.Remove(temp.Name()) }()
	data, err := newGenerator(key, spellicon.Options{Model: *model, Timeout: *timeout}).Generate(ctx, *prompt)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if _, err = temp.Write(data); err != nil {
		return err
	}
	if err = temp.Chmod(0644); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	if *overwrite {
		return os.Rename(temp.Name(), *out)
	}
	// Link publishes complete bytes atomically without replacing a file created
	// by another process while generation was running.
	return os.Link(temp.Name(), *out)
}
