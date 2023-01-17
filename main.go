package main

import (
	_ "embed"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"github.com/faiface/beep/wav"
	"github.com/flopp/go-findfont"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Music struct {
	AudioFormat
	a                    fyne.App
	w                    fyne.Window
	done                 chan bool
	ctrl                 *beep.Ctrl
	PlayButton           *widget.Button
	StopButton           *widget.Button
	AddButton            *widget.Button
	DoneButton           *widget.Button
	label                *widget.Label
	TimeLabel            *widget.Label
	entry                *widget.Entry
	FilePathDir          *widget.Label
	OpenFile             *dialog.FileDialog
	OpenFileButton       *widget.Button
	ProgressBar          *widget.ProgressBar
	Slider               *widget.Slider
	VolumeButton         *widget.Button
	MusicPathList        binding.StringList
	MusicPathListId      int
	WidgetListPath       *widget.List // 路径
	WidgetListHideButton *widget.Button
	Container            *fyne.Container
}

type AudioFormat struct {
	streamer beep.StreamSeekCloser
	format   beep.Format
	volume   *effects.Volume
	err      error
}

var music Music

func init() {
	paths := findfont.List()
	for _, path := range paths {
		if strings.Contains(path, "simkai.ttf") {
			os.Setenv("FYNE_FONT", path)
			break
		}
	}
}

func (music *Music) GetFileList() *widget.List {
	PathList := []string{} // 路径
	NameList := []string{} // 歌名

	if music.FilePathDir.Text == "" {
		music.FilePathDir.Text = "."
	}

	dir, err := os.ReadDir(music.FilePathDir.Text)
	if err != nil {
		return nil
	}

	for _, file := range dir {
		if music.FilePathDir.Text == "." {
			path := strings.Trim(string(filepath.Separator), "\\") + file.Name()
			ext := filepath.Ext(path)
			if ext == ".mp3" || ext == ".flac" || ext == ".wav" || ext == ".ogg" {
				PathList = append(PathList, path)
				NameList = append(NameList, path)
			}
		} else {
			path := strings.Trim(music.FilePathDir.Text+string(filepath.Separator), "\\") + file.Name()
			name := strings.Trim(string(filepath.Separator), "\\") + file.Name()
			ext := filepath.Ext(path)
			if ext == ".mp3" || ext == ".flac" || ext == ".wav" || ext == ".ogg" {
				PathList = append(PathList, path)
				NameList = append(NameList, name)
			}
		}
	}

	NameListData := binding.NewStringList()
	err = NameListData.Set(NameList)
	if err != nil {
		return nil
	}

	PathListData := binding.NewStringList()
	err = PathListData.Set(PathList)
	if err != nil {
		return nil
	}

	music.Stop()
	music.MusicPathListId = 0
	music.MusicPathList = nil
	music.MusicPathList = PathListData

	data := widget.NewListWithData(NameListData, func() fyne.CanvasObject {
		return widget.NewLabel("                                ")
	}, func(item binding.DataItem, object fyne.CanvasObject) {
		lbl := object.(*widget.Label)
		s := item.(binding.String)
		ss, _ := s.Get()
		lbl.SetText(ss)
	})

	return data
}

func (audioFormat *AudioFormat) Play() {
	if music.ctrl != nil {
		if !music.ctrl.Paused {
			music.PlayButton.SetIcon(theme.MediaPlayIcon())
		} else {
			music.PlayButton.SetIcon(theme.MediaPauseIcon())
		}

		speaker.Lock()
		music.ctrl.Paused = !music.ctrl.Paused
		speaker.Unlock()
	} else {
		music.PlayButton.SetIcon(theme.MediaPauseIcon())
		go func() {
			open, err := os.Open(music.entry.Text)
			if err != nil {
				music.label.SetText(err.Error())
				return
			}

			switch filepath.Ext(music.entry.Text) {
			case ".mp3":
				music.AudioFormat.streamer, music.AudioFormat.format, music.AudioFormat.err = mp3.Decode(open)
				if err != nil {
					music.label.SetText(err.Error())
					music.PlayNext()
				}
				break
			case ".wav":
				music.AudioFormat.streamer, music.AudioFormat.format, music.AudioFormat.err = wav.Decode(open)
				if err != nil {
					music.label.SetText(err.Error())
					music.PlayNext()
				}
				break
			case ".flac":
				music.AudioFormat.streamer, music.AudioFormat.format, music.AudioFormat.err = flac.Decode(open)
				if err != nil {
					music.label.SetText(err.Error())
					music.PlayNext()
				}
				break
			case ".ogg":
				music.AudioFormat.streamer, music.AudioFormat.format, music.AudioFormat.err = vorbis.Decode(open)
				if err != nil {
					music.label.SetText(err.Error())
					music.PlayNext()
				}
				break
			}

			defer music.AudioFormat.streamer.Close()

			speaker.Init(music.AudioFormat.format.SampleRate, music.AudioFormat.format.SampleRate.N(time.Second/10))
			music.ProgressBar.Max = float64(music.AudioFormat.streamer.Len())

			music.ctrl = &beep.Ctrl{
				Streamer: beep.Loop(-1, music.AudioFormat.streamer),
				Paused:   false,
			}

			music.AudioFormat.volume = &effects.Volume{
				Streamer: music.ctrl,
				Base:     2,
				Volume:   0,
				Silent:   false,
			}

			//speaker.Play(music.ctrl)
			speaker.Play(music.AudioFormat.volume)

			for {
				select {
				case <-music.done:
					speaker.Clear()
					return
				case <-time.After(time.Second):
					music.ProgressBar.SetValue(float64(music.AudioFormat.streamer.Position()))
					speaker.Lock()
					pos := music.AudioFormat.format.SampleRate.D(music.AudioFormat.streamer.Position())
					length := music.AudioFormat.format.SampleRate.D(music.AudioFormat.streamer.Len())
					music.TimeLabel.SetText(fmt.Sprintf("%v/%v", pos.Round(time.Second), length.Round(time.Second)))
					speaker.Unlock()
				}
			}
		}()
	}
}

func (audioFormat *AudioFormat) Stop() {
	if music.ctrl == nil {
		return
	}
	music.done <- false
	music.ctrl = nil
	music.label.SetText("")
	music.TimeLabel.SetText("")
	music.ProgressBar.SetValue(0)
	music.PlayButton.SetIcon(theme.MediaPlayIcon())
}

// 上一首
func (audioFormat *AudioFormat) PlayPrevious() {
	if music.MusicPathListId > 0 {
		music.MusicPathListId = music.MusicPathListId - 1
		value, err := music.MusicPathList.GetValue(music.MusicPathListId)
		if err != nil {
			return
		}
		music.WidgetListPath.Select(music.MusicPathListId)
		music.entry.SetText(value)
	} else {
		music.MusicPathListId = music.MusicPathList.Length() - 1
		value, err := music.MusicPathList.GetValue(music.MusicPathListId)
		if err != nil {
			return
		}
		music.WidgetListPath.Select(music.MusicPathListId)
		music.entry.SetText(value)
	}
	//music.AudioFormat.Stop()
	//music.AudioFormat.Play()
}

// 下一首
func (audioFormat *AudioFormat) PlayNext() {
	if music.MusicPathListId < music.MusicPathList.Length()-1 {
		music.MusicPathListId = music.MusicPathListId + 1
		value, err := music.MusicPathList.GetValue(music.MusicPathListId)
		if err != nil {
			return
		}
		music.WidgetListPath.Select(music.MusicPathListId)
		music.entry.SetText(value)
	} else {
		music.MusicPathListId = 0
		value, err := music.MusicPathList.GetValue(music.MusicPathListId)
		if err != nil {
			return
		}
		music.WidgetListPath.Select(music.MusicPathListId)
		music.entry.SetText(value)
	}
	//music.AudioFormat.Stop()
	//music.AudioFormat.Play()
}

//go:embed icon.png
var icon []byte

//go:embed background.jpg
var background []byte

func main() {
	music.a = app.NewWithID("music.fyne.com")
	music.a.SetIcon(fyne.NewStaticResource("icon", icon))
	music.a.Settings().SetTheme(customTheme{"Dark"})
	music.w = music.a.NewWindow("Go音乐")

	music.OpenFile = dialog.NewFileOpen(func(closer fyne.URIReadCloser, err error) {
		if err != nil {
			return
		}

		if closer == nil {
			return
		}

		music.entry.SetText(closer.URI().Path())

		// fmt.Println(music.entry.Text)

		music.FilePathDir.Text = strings.Trim(closer.URI().Path(), closer.URI().Name())

		// fmt.Println(music.FilePathDir.Text)

		music.WidgetListPath = music.GetFileList()

		// 重新绑定一下
		music.WidgetListPath.OnSelected = func(id widget.ListItemID) {
			music.AudioFormat.Stop()
			value, err := music.MusicPathList.GetValue(id)
			music.MusicPathListId = id
			if err != nil {
				return
			}
			// fmt.Println("ok")
			//fmt.Println(value)
			music.entry.SetText(value)
			music.AudioFormat.Play()
		}

		box := container.NewHBox(music.DoneButton, music.PlayButton, music.AddButton, music.TimeLabel, music.StopButton, music.VolumeButton, music.WidgetListHideButton)
		top := container.NewVBox(music.entry, music.ProgressBar, container.NewCenter(box))
		border := container.NewBorder(top, music.label, nil, nil, widget.NewLabel(""))
		music.Container = container.NewBorder(nil, nil, music.WidgetListPath, nil, border)
		music.w.SetContent(container.NewMax(canvas.NewImageFromResource(fyne.NewStaticResource("background", background)), music.Container))
	}, music.w)

	music.FilePathDir = widget.NewLabel("")
	music.WidgetListPath = music.GetFileList()

	music.OpenFile.SetFilter(storage.NewExtensionFileFilter([]string{".mp3", ".flac", ".ogg", ".wav"}))
	music.OpenFile.Resize(fyne.NewSize(1080, 720))

	music.OpenFileButton = widget.NewButtonWithIcon("", theme.FileAudioIcon(), func() {
		music.OpenFile.Show()
	})

	music.OpenFileButton.Importance = widget.LowImportance

	music.Slider = widget.NewSlider(-10, 10)
	music.Slider.SetValue(0)
	music.Slider.Orientation = widget.Vertical
	music.Slider.OnChanged = func(f float64) {
		if music.AudioFormat.volume == nil {
			return
		}
		//fmt.Println(f)
		speaker.Lock()
		music.AudioFormat.volume.Volume = f
		speaker.Unlock()
	}

	music.VolumeButton = widget.NewButtonWithIcon("", theme.VolumeUpIcon(), func() {
		widget.ShowPopUpAtPosition(music.Slider, fyne.CurrentApp().Driver().CanvasForObject(music.VolumeButton),
			music.VolumeButton.Position().Add(fyne.NewDelta(music.VolumeButton.Size().Width/2, 0)))
	})

	music.TimeLabel = widget.NewLabel("")
	music.ProgressBar = widget.NewProgressBar()
	music.ProgressBar.Min = 0
	music.ProgressBar.Max = 100
	music.ProgressBar.Value = 0

	music.done = make(chan bool)
	music.label = widget.NewLabel("")
	music.entry = widget.NewEntry()
	music.entry.Text = "文爱 - CG、贺敬轩.mp3"
	music.entry.ActionItem = music.OpenFileButton

	music.PlayButton = widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
		music.AudioFormat.Play()
	})

	music.StopButton = widget.NewButtonWithIcon("", theme.MediaStopIcon(), func() {
		music.AudioFormat.Stop()
	})

	music.WidgetListPath.OnSelected = func(id widget.ListItemID) {
		music.AudioFormat.Stop()
		value, err := music.MusicPathList.GetValue(id)
		music.MusicPathListId = id
		if err != nil {
			return
		}
		music.entry.SetText(value)
		music.AudioFormat.Play()
	}

	music.MusicPathList.AddListener(binding.NewDataListener(func() {
		speaker.Clear()
	}))

	// 上一首
	music.DoneButton = widget.NewButtonWithIcon("", theme.MediaSkipPreviousIcon(), func() {
		music.PlayPrevious()
	})

	// 下一首
	music.AddButton = widget.NewButtonWithIcon("", theme.MediaSkipNextIcon(), func() {
		music.PlayNext()
	})

	music.WidgetListHideButton = widget.NewButtonWithIcon("", theme.ListIcon(), func() {
		music.WidgetListPath.Hidden = !music.WidgetListPath.Hidden
	})

	box := container.NewHBox(music.DoneButton, music.PlayButton, music.AddButton, music.TimeLabel, music.StopButton, music.VolumeButton, music.WidgetListHideButton)
	top := container.NewVBox(music.entry, music.ProgressBar, container.NewCenter(box))
	border := container.NewBorder(top, music.label, nil, nil, widget.NewLabel(""))
	music.Container = container.NewBorder(nil, nil, music.WidgetListPath, nil, border)

	//music.w.SetContent(container.NewMax(music.Container, canvas.NewImageFromResource(fyne.NewStaticResource("background", background))))
	music.w.SetContent(container.NewMax(canvas.NewImageFromResource(fyne.NewStaticResource("background", background)), music.Container))
	music.w.Resize(fyne.NewSize(1080, 720))
	music.w.CenterOnScreen()
	music.w.ShowAndRun()
}

type customTheme struct {
	Theme string
}

func (m customTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch m.Theme {
	case "Dark":
		variant = theme.VariantDark
		switch name {
		case theme.ColorNameDisabled:
			return color.NRGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xff}
		case theme.ColorNameBackground:
			return color.NRGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xff}
		case theme.ColorNameButton:
			return color.NRGBA{R: 0x44, G: 0x44, B: 0x44, A: 0xff}
		case theme.ColorNameDisabledButton:
			return color.NRGBA{R: 0x26, G: 0x26, B: 0x26, A: 0xff}
		case theme.ColorNameOverlayBackground:
			return color.NRGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xff}
		case theme.ColorNameMenuBackground:
			return color.NRGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xff}
		}

	case "Light":
		variant = theme.VariantLight
		switch name {
		case theme.ColorNameDisabled:
			return color.NRGBA{R: 0xab, G: 0xab, B: 0xab, A: 0xff}
		case theme.ColorNameInputBorder:
			return color.NRGBA{R: 0xf3, G: 0xf3, B: 0xf3, A: 0xff}
		case theme.ColorNameDisabledButton:
			return color.NRGBA{R: 0xe5, G: 0xe5, B: 0xe5, A: 0xff}
		}
	}

	return theme.DefaultTheme().Color(name, variant)
}

func (m customTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m customTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m customTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
