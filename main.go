package main

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/nfnt/resize"
)

var (
	myApp         fyne.App
	myWin         fyne.Window
	games         []Game
	grid          *fyne.Container
	thumbWidth    float32 = 150
	thumbHeight   float32 = 225
	selectedIndex int     = -1
)

type tappableImage struct {
	widget.BaseWidget
	image          *canvas.Image
	onTap          func()
	onDoubleTap    func()
	onRightContext func(*fyne.PointEvent)
}

func newTappableImage(img *canvas.Image, onTap func(), onDoubleTap func(), onRight func(*fyne.PointEvent)) *tappableImage {
	t := &tappableImage{image: img, onTap: onTap, onDoubleTap: onDoubleTap, onRightContext: onRight}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableImage) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.image)
}

func (t *tappableImage) Tapped(_ *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tappableImage) TappedSecondary(pe *fyne.PointEvent) {
	if t.onRightContext != nil {
		t.onRightContext(pe)
	}
}

func (t *tappableImage) DoubleTapped(_ *fyne.PointEvent) {
	if t.onDoubleTap != nil {
		t.onDoubleTap()
	}
}

func main() {
	myApp = app.NewWithID("com.umu-launcher.umu-front")
	myWin = myApp.NewWindow("umu-front")
	myWin.Resize(fyne.NewSize(800, 600))

	var err error
	games, err = loadGames()
	if err != nil {
		log.Println("Error loading games:", err)
	}

	thumbWidth = float32(myApp.Preferences().FloatWithFallback("thumbWidth", float64(thumbWidth)))
	thumbHeight = float32(myApp.Preferences().FloatWithFallback("thumbHeight", float64(thumbHeight)))

	grid = container.NewGridWrap(fyne.NewSize(thumbWidth, thumbHeight))
	refreshGrid()

	addButton := widget.NewButton("Add Game", showAddGameDialog)

	zoomInBtn := widget.NewButton("+", func() {
		thumbWidth *= 1.2
		thumbHeight *= 1.2
		myApp.Preferences().SetFloat("thumbWidth", float64(thumbWidth))
		myApp.Preferences().SetFloat("thumbHeight", float64(thumbHeight))
		grid.Layout = layout.NewGridWrapLayout(fyne.NewSize(thumbWidth, thumbHeight))
		grid.Refresh()
	})
	zoomOutBtn := widget.NewButton("-", func() {
		thumbWidth /= 1.2
		thumbHeight /= 1.2
		myApp.Preferences().SetFloat("thumbWidth", float64(thumbWidth))
		myApp.Preferences().SetFloat("thumbHeight", float64(thumbHeight))
		grid.Layout = layout.NewGridWrapLayout(fyne.NewSize(thumbWidth, thumbHeight))
		grid.Refresh()
	})

	mainLayout := container.NewBorder(
		container.NewHBox(zoomOutBtn, zoomInBtn, layout.NewSpacer(), addButton),
		nil, nil, nil,
		container.NewVScroll(grid),
	)

	myWin.SetContent(mainLayout)
	myWin.ShowAndRun()
}

func refreshGrid() {
	grid.Objects = nil
	for i, g := range games {
		g := g // specific instance for closure
		idx := i

		img := canvas.NewImageFromFile(g.ImageURL)
		img.FillMode = canvas.ImageFillContain

		highlight := canvas.NewRectangle(theme.SelectionColor())
		if selectedIndex != idx {
			highlight.Hide()
		}

		tImage := newTappableImage(img, func() {
			if selectedIndex != idx {
				selectedIndex = idx
				refreshGrid()
			}
		}, func() {
			launchGame(g)
		}, func(pe *fyne.PointEvent) {
			menu := fyne.NewMenu("",
				fyne.NewMenuItem("Edit", func() {
					showEditGameDialog(idx, g)
				}),
				fyne.NewMenuItem("Delete", func() {
					dialog.ShowConfirm("Delete Game", fmt.Sprintf("Are you sure you want to delete %s?", g.Name), func(b bool) {
						if b {
							games = append(games[:idx], games[idx+1:]...)
							if selectedIndex == idx {
								selectedIndex = -1
							} else if selectedIndex > idx {
								selectedIndex--
							}
							saveGames(games)
							refreshGrid()
						}
					}, myWin)
				}),
			)
			widget.ShowPopUpMenuAtPosition(menu, myWin.Canvas(), pe.AbsolutePosition)
		})

		lblName := widget.NewLabelWithStyle(g.Name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		lblName.Wrapping = fyne.TextWrapWord

		item := container.NewBorder(nil, lblName, nil, nil, container.NewMax(highlight, container.NewPadded(tImage)))

		grid.Add(item)
	}
	grid.Refresh()
}

func launchGame(g Game) {
	fmt.Println("Launching", g.Name)

	cmd := exec.Command("umu-run", g.ExecPath)
	cmd.Env = os.Environ()

	if g.Prefix != "" {
		cmd.Env = append(cmd.Env, "WINEPREFIX="+g.Prefix)
	}
	if g.ProtonVer != "" {
		home, _ := os.UserHomeDir()
		protonPath := filepath.Join(home, ".steam/steam/compatibilitytools.d", g.ProtonVer)
		cmd.Env = append(cmd.Env, "PROTONPATH="+protonPath)
	}
	if g.ID != "" {
		cmd.Env = append(cmd.Env, "GAMEID=umu-"+g.ID)
		cmd.Env = append(cmd.Env, "STORE=steam")
	}
	if g.DLLOverrides != "" {
		cmd.Env = append(cmd.Env, "WINEDLLOVERRIDES="+g.DLLOverrides)
	}

	// We attach stdout/stderr to the main process so we can debug umu-run
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		dialog.ShowError(err, myWin)
		return
	}
	// We don't wait for it to finish
}

func processCustomImage(srcPath string) (string, error) {
	file, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return "", err
	}

	m := resize.Resize(600, 900, img, resize.Lanczos3)

	fileName := fmt.Sprintf("custom_%d.jpg", time.Now().UnixNano())
	destPath := filepath.Join(getImagesDir(), fileName)

	out, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	return destPath, jpeg.Encode(out, m, nil)
}

func showAddGameDialog() {
	nameEntry := widget.NewEntry()
	execPathEntry := widget.NewEntry()
	prefixEntry := widget.NewEntry()
	imgEntry := widget.NewEntry()
	dllOverridesEntry := widget.NewEntry()

	protonVersions := getProtonVersions()
	protonSelect := widget.NewSelect(protonVersions, nil)
	if len(protonVersions) > 0 {
		protonSelect.SetSelected(protonVersions[0])
	}

	execBtn := widget.NewButton("Browse", func() {
		// show file open dialog
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if reader != nil {
				execPathEntry.SetText(reader.URI().Path())
			}
		}, myWin)
		fd.Resize(fyne.NewSize(800, 600))
		fd.Show()
	})

	prefixBtn := widget.NewButton("Browse", func() {
		// show folder open dialog
		fd := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				prefixEntry.SetText(uri.Path())
			}
		}, myWin)
		fd.Resize(fyne.NewSize(800, 600))
		fd.Show()
	})

	searchBtn := widget.NewButton("Search Steam Info", func() {
		if nameEntry.Text == "" {
			return
		}
		items, err := searchSteamGame(nameEntry.Text)
		if err != nil {
			dialog.ShowError(err, myWin)
			return
		}

		if len(items) == 0 {
			dialog.ShowInformation("Not Found", "No games found on Steam.", myWin)
			return
		}

		// Simple list of names
		var opts []string
		itemMap := make(map[string]SteamSearchItem)
		for _, item := range items {
			label := fmt.Sprintf("%s (ID: %d)", item.Name, item.ID)
			opts = append(opts, label)
			itemMap[label] = item
		}

		var selectedItem SteamSearchItem
		sel := widget.NewSelect(opts, func(s string) {
			selectedItem = itemMap[s]
		})

		dialog.ShowCustomConfirm("Select Game", "Select", "Cancel", container.NewVBox(sel), func(ok bool) {
			if ok && selectedItem.ID != 0 {
				nameEntry.SetText(selectedItem.Name)
				// Create id and schedule thumbnail download later
				dialog.ShowInformation("Selected", fmt.Sprintf("Selected %s", selectedItem.Name), myWin)
			}
		}, myWin)
	})

	imgBtn := widget.NewButton("Browse", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if reader != nil {
				imgEntry.SetText(reader.URI().Path())
			}
		}, myWin)
		fd.Resize(fyne.NewSize(800, 600))
		fd.Show()
	})

	form := dialog.NewForm("Add New Game", "Save", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Name (or search)", container.NewBorder(nil, nil, nil, searchBtn, nameEntry)),
		widget.NewFormItem("Executable", container.NewBorder(nil, nil, nil, execBtn, execPathEntry)),
		widget.NewFormItem("Wine Prefix", container.NewBorder(nil, nil, nil, prefixBtn, prefixEntry)),
		widget.NewFormItem("Proton Version", protonSelect),
		widget.NewFormItem("Custom Image", container.NewBorder(nil, nil, nil, imgBtn, imgEntry)),
		widget.NewFormItem("DLL Overrides", dllOverridesEntry),
	}, func(ok bool) {
		if ok {
			saveNewGame(nameEntry.Text, execPathEntry.Text, prefixEntry.Text, protonSelect.Selected, imgEntry.Text, dllOverridesEntry.Text)
		}
	}, myWin)
	form.Resize(fyne.NewSize(600, 500))
	form.Show()
}

func showEditGameDialog(index int, g Game) {
	nameEntry := widget.NewEntry()
	nameEntry.SetText(g.Name)

	execPathEntry := widget.NewEntry()
	execPathEntry.SetText(g.ExecPath)

	prefixEntry := widget.NewEntry()
	prefixEntry.SetText(g.Prefix)

	imgEntry := widget.NewEntry()
	imgEntry.SetText(g.ImageURL)

	dllOverridesEntry := widget.NewEntry()
	dllOverridesEntry.SetText(g.DLLOverrides)

	protonVersions := getProtonVersions()
	protonSelect := widget.NewSelect(protonVersions, nil)
	if g.ProtonVer != "" {
		protonSelect.SetSelected(g.ProtonVer)
	} else if len(protonVersions) > 0 {
		protonSelect.SetSelected(protonVersions[0])
	}

	execBtn := widget.NewButton("Browse", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if reader != nil {
				execPathEntry.SetText(reader.URI().Path())
			}
		}, myWin)
		fd.Resize(fyne.NewSize(800, 600))
		fd.Show()
	})

	prefixBtn := widget.NewButton("Browse", func() {
		fd := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				prefixEntry.SetText(uri.Path())
			}
		}, myWin)
		fd.Resize(fyne.NewSize(800, 600))
		fd.Show()
	})

	imgBtn := widget.NewButton("Browse", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if reader != nil {
				imgEntry.SetText(reader.URI().Path())
			}
		}, myWin)
		fd.Resize(fyne.NewSize(800, 600))
		fd.Show()
	})

	form := dialog.NewForm("Edit Game", "Save", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Executable", container.NewBorder(nil, nil, nil, execBtn, execPathEntry)),
		widget.NewFormItem("Wine Prefix", container.NewBorder(nil, nil, nil, prefixBtn, prefixEntry)),
		widget.NewFormItem("Proton Version", protonSelect),
		widget.NewFormItem("Custom Image", container.NewBorder(nil, nil, nil, imgBtn, imgEntry)),
		widget.NewFormItem("DLL Overrides", dllOverridesEntry),
	}, func(ok bool) {
		if ok {
			imgURL := imgEntry.Text
			if imgURL != g.ImageURL && imgURL != "" && filepath.Dir(imgURL) != getImagesDir() {
				if newPath, err := processCustomImage(imgURL); err == nil {
					imgURL = newPath
				} else {
					log.Println("Error processing image:", err)
				}
			}

			// update existing instance
			games[index].Name = nameEntry.Text
			games[index].ExecPath = execPathEntry.Text
			games[index].Prefix = prefixEntry.Text
			games[index].ProtonVer = protonSelect.Selected
			games[index].ImageURL = imgURL
			games[index].DLLOverrides = dllOverridesEntry.Text

			saveGames(games)
			refreshGrid()
		}
	}, myWin)
	form.Resize(fyne.NewSize(600, 530))
	form.Show()
}

func saveNewGame(name, execPath, prefix, protonVer, customImg, dllOverrides string) {
	// Let's do a search just to see if we can get an ID natively if they didn't use the search button perfectly
	var gameID string
	var imgPath string

	if customImg != "" {
		if newPath, err := processCustomImage(customImg); err == nil {
			imgPath = newPath
		} else {
			log.Println("Error processing custom image:", err)
		}
	}

	if imgPath == "" {
		items, err := searchSteamGame(name)
		if err == nil && len(items) > 0 {
			gameID = fmt.Sprintf("%d", items[0].ID)

			// Attempt to download thumbnail
			dest := filepath.Join(getImagesDir(), gameID+".jpg")
			err := downloadThumbnail(items[0].ID, dest)
			if err == nil {
				imgPath = dest
			}
		}
	} else if gameID == "" {
		// Still try to get an ID for UMU fixes
		items, err := searchSteamGame(name)
		if err == nil && len(items) > 0 {
			gameID = fmt.Sprintf("%d", items[0].ID)
		}
	}

	game := Game{
		ID:           gameID,
		Name:         name,
		ExecPath:     execPath,
		Prefix:       prefix,
		ProtonVer:    protonVer,
		ImageURL:     imgPath,
		DLLOverrides: dllOverrides,
	}

	games = append(games, game)
	saveGames(games)
	refreshGrid()
}
