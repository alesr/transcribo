package scriber

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alesr/whisperclient"
)

const sampleRate = "5200"

type Input interface {
	Name() string
	OutputType() string
	Language() string
	Data() io.ReadCloser
}

type Output struct {
	Name string
	Text []byte
}

type whisperClient interface {
	TranscribeAudio(ctx context.Context, in whisperclient.TranscribeAudioInput) ([]byte, error)
}

type Scriber struct {
	logger        *slog.Logger
	whisperClient whisperClient
	resultsCh     chan Output
}

func New(logger *slog.Logger, whisperCli whisperClient) *Scriber {
	return &Scriber{
		logger:        logger.WithGroup("scriber"),
		whisperClient: whisperCli,
		resultsCh:     make(chan Output, 10),
	}
}

func (s *Scriber) Process(ctx context.Context, in Input) error {
	s.logger.Info("Processing file", slog.String("name", in.Name()))

	data, err := io.ReadAll(in.Data())
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	defer in.Data().Close()

	cmd := exec.Command(
		"ffmpeg", "-y",
		"-i", "pipe:0",
		"-vn",
		"-acodec", "pcm_s16le",
		"-ar", sampleRate,
		"-ac", "2",
		"-b:a", "32k",
		"-f", "wav",
		"pipe:1",
	)

	cmd.Stdin = bytes.NewReader(data)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = os.Stderr

	s.logger.Info("Running ffmpeg", slog.String("file", in.Name()))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	s.logger.Info("Transcribing audio", slog.String("file", in.Name()))

	text, err := s.whisperClient.TranscribeAudio(ctx, whisperclient.TranscribeAudioInput{
		Name:     in.Name(),
		Language: in.Language(),
		Format:   in.OutputType(),
		Data:     &outBuf,
	})
	if err != nil {
		return fmt.Errorf("transcription failed: %w", err)
	}

	s.resultsCh <- Output{
		Name: strings.Replace(
			in.Name(),
			filepath.Ext(in.Name()),
			"."+in.OutputType(), 1,
		), // foo.mp4 -> foo.srt
		Text: text,
	}

	s.logger.Info("Processing complete", slog.String("file", in.Name()))
	return nil
}

func (s *Scriber) Collect() <-chan Output {
	return s.resultsCh
}
