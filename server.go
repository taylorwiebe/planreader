package main

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed web/*
var webFiles embed.FS

type ReaderDocument struct {
	FileName  string                  `json:"file_name"`
	Narration Narration               `json:"narration"`
	Sources   []RenderedSourceSection `json:"sources"`
}

type RenderedSourceSection struct {
	ID      string `json:"id"`
	Heading string `json:"heading"`
	Level   int    `json:"level"`
	HTML    string `json:"html"`
}

func renderSourceSections(sections []SourceSection) ([]RenderedSourceSection, error) {
	markdown := goldmark.New(goldmark.WithExtensions(extension.GFM))
	rendered := make([]RenderedSourceSection, 0, len(sections))
	for _, section := range sections {
		var output bytes.Buffer
		if err := markdown.Convert([]byte(section.Markdown), &output); err != nil {
			return nil, fmt.Errorf("rendering source section %q: %w", section.Heading, err)
		}
		rendered = append(rendered, RenderedSourceSection{
			ID:      section.ID,
			Heading: section.Heading,
			Level:   section.Level,
			HTML:    output.String(),
		})
	}
	return rendered, nil
}

func newReaderHandler(document ReaderDocument, token string) http.Handler {
	prefix := "/reader/" + token + "/"
	data, err := json.Marshal(document)
	if err != nil {
		panic(fmt.Sprintf("encoding reader document: %v", err))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asset, ok := strings.CutPrefix(r.URL.Path, prefix)
		if r.Method != http.MethodGet || !ok {
			http.NotFound(w, r)
			return
		}
		if asset == "" {
			asset = "index.html"
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")

		if asset == "data.json" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_, _ = w.Write(data)
			return
		}

		content, err := fs.ReadFile(webFiles, "web/"+asset)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		switch {
		case strings.HasSuffix(asset, ".html"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case strings.HasSuffix(asset, ".css"):
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case strings.HasSuffix(asset, ".js"):
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		}
		_, _ = w.Write(content)
	})
}

func startReaderServer(document ReaderDocument) (string, *http.Server, error) {
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", nil, fmt.Errorf("creating local access token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("starting local reader: %w", err)
	}
	server := &http.Server{
		Handler:           newReaderHandler(document, token),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		_ = server.Serve(listener)
	}()

	url := fmt.Sprintf("http://%s/reader/%s/", listener.Addr().String(), token)
	return url, server, nil
}

func openBrowser(url string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command, args = "open", []string{url}
	case "windows":
		command, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		command, args = "xdg-open", []string{url}
	}
	cmd := exec.Command(command, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening browser: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
