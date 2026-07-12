package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/notioncodex/gateway/internal/notion/discovery/picker"
	"github.com/notioncodex/gateway/internal/notion/fixtures"
	"github.com/notioncodex/gateway/internal/notion/protocol"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 2 {
		return usageError()
	}
	switch args[0] {
	case "probe-fixture":
		return probeFixture(args[1])
	case "decode-normalized-stream":
		return decodeNormalizedStream(args[1])
	case "models-probe-fixture":
		return modelsProbeFixture(args[1])
	default:
		return usageError()
	}
}

func usageError() error {
	return errors.New("usage: notion-codex <probe-fixture|decode-normalized-stream|models-probe-fixture> <file>")
}

func probeFixture(name string) error {
	file, err := openRegularFile(name)
	if err != nil {
		return err
	}
	defer file.Close()
	envelope, err := fixtures.Load(file)
	if err != nil {
		return err
	}
	fmt.Printf("fixture valid: direction=%s transport=%s payload_bytes=%d\n", envelope.Direction, envelope.Transport, len(envelope.Payload))
	return nil
}

func decodeNormalizedStream(name string) error {
	file, err := openRegularFile(name)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := protocol.NewStreamDecoder(file, protocol.DefaultMaxFrameBytes)
	for sequence := 0; ; sequence++ {
		event, err := decoder.Next(context.Background())
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		fmt.Printf("sequence=%d kind=%s text_bytes=%d error=%t\n", sequence, event.Kind, len(event.Text), event.Err != nil)
	}
}

func modelsProbeFixture(name string) error {
	file, err := openRegularFile(name)
	if err != nil {
		return err
	}
	defer file.Close()
	envelope, err := fixtures.Load(file)
	if err != nil {
		return err
	}
	models, err := picker.Parse(envelope.Payload, "<account-id>", "<workspace-id>", time.Now())
	if err != nil {
		return err
	}
	for _, model := range models {
		fmt.Printf("%s\t%s\t%s\t%s\n", model.PublicID, model.DisplayName, model.Provider, model.Status)
	}
	return nil
}

func openRegularFile(name string) (*os.File, error) {
	path, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("input must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}
