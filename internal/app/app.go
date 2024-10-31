package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/alesr/transcribo/internal/scriber"
)

const (
	appName     = "transcribo"
	fileMaxSize = 1024 * 1024 * 1024 // 1 GB
)

var (
	winSize   = windowSize{800, 600}
	validExts = map[string]struct{}{
		".mp4":  {},
		".mp3":  {},
		".wav":  {},
		".webm": {},
		".avi":  {},
	}
	extFilter = []string{".mp4", ".mp3", ".wav", ".webm", ".avi"}
)

type (
	scriberSvc interface {
		Process(ctx context.Context, in scriber.Input) error
		Collect() <-chan scriber.Output
	}

	windowSize struct {
		width, height float32
	}

	inputFile struct {
		name       string
		language   string
		outputType string
		data       io.ReadCloser
	}

	progressStatus struct {
		filename string
		stage    string
		err      error
	}
)

type scriberInput struct {
	name       string
	language   string
	outputType string
	data       io.ReadCloser
}

func (in *scriberInput) Name() string        { return in.name }
func (in *scriberInput) Language() string    { return in.language }
func (in *scriberInput) OutputType() string  { return in.outputType }
func (in *scriberInput) Data() io.ReadCloser { return in.data }

type App struct {
	mu            sync.RWMutex
	logger        *slog.Logger
	name          string
	fyneApp       fyne.App
	theme         guiTheme
	window        fyne.Window
	selectedFiles []inputFile
	scriberSvc    scriberSvc
	ctx           context.Context
	cancel        context.CancelFunc

	components struct {
		mainContent      *fyne.Container
		fileList         *fyne.Container
		processContainer *fyne.Container
		progressBar      *widget.ProgressBarInfinite
		statusLabel      *widget.TextGrid
		resultsList      *fyne.Container
		uploadBtn        *widget.Button
		processBtn       *widget.Button
		statusCh         chan progressStatus
	}
}

func New(logger *slog.Logger, scriberSvc scriberSvc) *App {
	ctx, cancel := context.WithCancel(context.Background())

	fApp := fyneapp.NewWithID(appName)

	t := guiTheme{variant: theme.VariantDark}
	fApp.Settings().SetTheme(&t)

	return &App{
		logger:        logger.WithGroup("app"),
		name:          appName,
		fyneApp:       fApp,
		theme:         t,
		selectedFiles: make([]inputFile, 0),
		scriberSvc:    scriberSvc,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (a *App) Run() {
	defer a.cancel()

	a.window = a.fyneApp.NewWindow(a.name)
	a.window.Resize(fyne.NewSize(winSize.width, winSize.height))
	a.window.CenterOnScreen()
	a.window.SetFixedSize(false)

	a.initComponents()

	mainScroll := container.NewVScroll(a.components.mainContent)
	content := container.NewPadded(mainScroll)
	a.window.SetContent(content)

	a.setupTheme()
	a.setupHeader()
	a.setupFileHandling()
	a.setupProcessing()
	a.setupResults()
	a.setupLayout()
	a.setupShortcuts()

	a.window.SetCloseIntercept(func() {
		a.cancel()
		close(a.components.statusCh)
		a.window.Close()
	})
	go a.handleStatus()
	a.window.ShowAndRun()
}

func (a *App) initComponents() {
	a.components.statusCh = make(chan progressStatus, 10)
	a.components.mainContent = container.NewVBox()
	a.components.fileList = container.NewVBox()
	a.components.processContainer = container.NewVBox()
	a.components.progressBar = widget.NewProgressBarInfinite()
	a.components.statusLabel = widget.NewTextGrid()
	a.components.resultsList = container.NewVBox()
	a.components.uploadBtn = widget.NewButtonWithIcon("Select Files", theme.FolderOpenIcon(), nil)
	a.components.processBtn = widget.NewButtonWithIcon("Process Files", theme.ConfirmIcon(), nil)

	a.components.progressBar.Hide()
	a.components.statusLabel.Hide()
	a.components.processContainer.Hide()
}

func (a *App) setupTheme() {
	themeToggle := widget.NewButton("", func() {
		if a.theme.variant == theme.VariantLight {
			a.theme.variant = theme.VariantDark
			a.fyneApp.Settings().SetTheme(&guiTheme{variant: theme.VariantDark})
			return
		}
		a.theme.variant = theme.VariantLight
		a.fyneApp.Settings().SetTheme(&guiTheme{variant: theme.VariantLight})
	})
	themeToggle.Icon = theme.ColorPaletteIcon()

	a.components.mainContent.Add(container.NewHBox(layout.NewSpacer(), themeToggle))
}

func (a *App) setupHeader() {
	headerTitle := widget.NewRichTextFromMarkdown("# " + appName)
	headerSubtitle := widget.NewRichTextFromMarkdown("## Generate subtitles and transcriptions")

	header := container.NewVBox(
		container.NewCenter(headerTitle),
		container.NewCenter(headerSubtitle),
		widget.NewSeparator(),
	)
	a.components.mainContent.Add(header)
}

func (a *App) setupFileHandling() {
	fileSelection := container.NewVBox(
		container.NewCenter(
			widget.NewRichTextFromMarkdown("Supported Formats: **MP4, MP3, WAV, WEBM, AVI _(max 1GB)_**"),
		),
		container.NewCenter(a.components.uploadBtn),
		widget.NewSeparator(),
		a.components.fileList,
	)
	a.setupUploadButton()
	a.components.mainContent.Add(fileSelection)
}

func (a *App) updateFileList() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.components.fileList.Objects = nil
	for i := range a.selectedFiles {
		f := a.selectedFiles[i]
		fileRow := container.NewHBox(
			widget.NewLabel(f.name),
			widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				a.removeFile(f.name)
			}),
		)
		a.components.fileList.Add(fileRow)
	}

	if len(a.selectedFiles) > 0 {
		a.components.processContainer.Show()
	} else {
		a.components.processContainer.Hide()
	}
	a.components.fileList.Refresh()
}

func (a *App) setupResults() {
	resultsHeader := widget.NewLabelWithStyle("Results", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	a.components.resultsList.Add(resultsHeader)
	a.components.resultsList.Hide()
	a.components.mainContent.Add(a.components.resultsList)
}

func (a *App) setupLayout() {
	a.components.mainContent.Objects = append(
		a.components.mainContent.Objects,
		a.components.progressBar,
		a.components.statusLabel,
		widget.NewSeparator(),
		a.components.resultsList,
	)
}

func (a *App) setupShortcuts() {
	a.window.Canvas().SetOnTypedKey(func(ke *fyne.KeyEvent) {
		if ke.Name == fyne.KeyEscape {
			a.cancel()
			close(a.components.statusCh)
			a.window.Close()
		}
	})
}

func (a *App) removeFile(name string) {
	a.mu.Lock()
	a.selectedFiles = slices.DeleteFunc(a.selectedFiles, func(v inputFile) bool {
		return v.name == name
	})
	a.mu.Unlock()
	a.updateFileList()
}

func (a *App) setupUploadButton() {
	a.components.uploadBtn.OnTapped = func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}

			if reader == nil {
				return
			}
			defer reader.Close()

			info, err := os.Stat(reader.URI().Path())
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}

			if info.Size() > fileMaxSize {
				dialog.ShowError(errors.New("file size exceeds 1GB limit"), a.window)
				return
			}

			ext := strings.ToLower(filepath.Ext(reader.URI().Path()))
			if _, ok := validExts[ext]; !ok {
				dialog.ShowError(errors.New("unsupported file format"), a.window)
				return
			}

			var buf bytes.Buffer
			if _, err := io.Copy(&buf, reader); err != nil {
				dialog.ShowError(fmt.Errorf("reading file: %w", err), a.window)
				return
			}

			a.mu.Lock()
			a.selectedFiles = append(a.selectedFiles, inputFile{
				name:       info.Name(),
				language:   "en",
				outputType: "srt",
				data:       io.NopCloser(bytes.NewReader(buf.Bytes())),
			})
			a.mu.Unlock()

			a.updateFileList()
		}, a.window)
		fd.SetFilter(storage.NewExtensionFileFilter(extFilter))
		fd.Show()
	}
}

func (a *App) handleStatus() {
	for status := range a.components.statusCh {
		a.updateStatus(status)
	}
}

func (a *App) updateStatus(status progressStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if status.err != nil {
		dialog.ShowError(fmt.Errorf("error processing file %s: %w", status.filename, status.err), a.window)
		return
	}

	statusText := fmt.Sprintf("File: %s\nStatus: %s", status.filename, status.stage)
	a.components.statusLabel.SetText(statusText)
	a.components.statusLabel.Refresh()

	if status.stage == "completed" {
		resultLabel := widget.NewLabel(fmt.Sprintf("Completed: %s", status.filename))
		a.components.resultsList.Add(resultLabel)
		a.components.resultsList.Refresh()
		a.components.resultsList.Show() // Ensure the results list is visible
	}
}

func (a *App) setupProcessing() {
	a.components.processBtn.OnTapped = func() {
		a.components.progressBar.Show()
		a.components.statusLabel.Show()
		go a.processFiles()
	}
	a.components.processContainer.Add(a.components.processBtn)
	a.components.mainContent.Add(a.components.processContainer)
}

func (a *App) processFiles() {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, file := range a.selectedFiles {
		a.components.statusCh <- progressStatus{filename: file.name, stage: "processing"}

		if err := a.scriberSvc.Process(a.ctx, &scriberInput{
			name:       file.name,
			language:   file.language,
			outputType: file.outputType,
			data:       file.data,
		}); err != nil {
			a.components.statusCh <- progressStatus{filename: file.name, stage: "failed", err: err}
			continue
		}
		a.components.statusCh <- progressStatus{filename: file.name, stage: "completed"}
	}
	a.components.progressBar.Hide()
}
