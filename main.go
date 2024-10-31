package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/alesr/httpclient"
	"github.com/alesr/transcribo/internal/app"
	"github.com/alesr/transcribo/internal/scriber"
	"github.com/alesr/whisperclient"
)

const whisperAIModel string = "whisper-1"

func main() {
	logger := slog.Default()

	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		logger.Error("OPENAI_API_KEY is required")
		os.Exit(1)
	}

	app.New(logger,
		scriber.New(
			logger,
			whisperclient.New(
				httpclient.New(
					httpclient.WithTimeout(10*time.Minute),
					httpclient.WithDialerTimeout(10*time.Second),
					httpclient.WithDialerKeepAlive(30*time.Second),
					httpclient.WithTLSHandshakeTimeout(10*time.Second),
					httpclient.WithResponseHeaderTimeout(30*time.Second),
					httpclient.WithIdleConnTimeout(30*time.Second),
					httpclient.WithMaxIdleConns(25),
					httpclient.WithForceHTTP2Disabled(),
				),
				openAIKey,
				whisperAIModel,
			),
		),
	).Run()
}
