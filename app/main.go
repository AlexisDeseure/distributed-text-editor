package main

import (
	"app/utils"
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const (
	// message type to be sent to controler
	MsgAppRequest string = "rqa" // request critical section
	MsgAppRelease string = "rla" // release critical section
	MsgAppDied    string = "apd" // app died
	// message type to be receive from controler
	MsgAppStartSc  string = "ssa" // start critical section
	MsgAppUpdate   string = "upa" // update critical section
	MsgAppShallDie string = "shd" // app shall die
)

const (
	TypeField string = "typ" // type of message
	UptField  string = "upt" // content of update for application
)

var outputDir *string = flag.String("o", "./output", "output directory")

// Interval in seconds between autosaves
const autoSaveInterval = 200 * time.Millisecond

// const autoSaveInterval = 2 * time.Second

var id *int = flag.Int("id", 0, "id of site")

var debug *bool = flag.Bool("debug", false, "enable debug mode (manual save)")
var saveTrigger = make(chan struct{})

var mutex = &sync.Mutex{}

var localSaveFilePath string

var lastText string
var sectionAccess bool = false
var sectionAccessRequested bool = false

func main() {

	// Parse command line arguments
	flag.Parse()
	if *id < 0 {
		display_e("Invalid site id")
		return
	}

	localSaveFilePath = fmt.Sprintf("%s/modifs_%d.log", *outputDir, *id)

	// Initialize the UI and get window and text area
	myWindow, textArea := initUI()

	lastText = textArea.Text

	go send(textArea)
	go receive(textArea, myWindow)

	// Display the window
	myWindow.ShowAndRun()
}

func send(textArea *widget.Entry) {
	var sndmsg string
	for {

		if *debug && !sectionAccessRequested {
			// Wait for the manual save trigger
			<-saveTrigger
		} else {
			// Wait for the autosave interval
			time.Sleep(autoSaveInterval)
		}

		sndmsg = ""
		mutex.Lock()
		cur := textArea.Text

		// Check if the controller has granted access to the critical section
		if sectionAccess {
			newTextDiffs := utils.ComputeDiffs(lastText, cur)
			newText := utils.ApplyDiffs(lastText, newTextDiffs)
			utils.SaveModifs(lastText, newText, localSaveFilePath)
			lastText = newText

			sndmsgBytes, err := json.Marshal(newTextDiffs)
			if err != nil {
				display_e("Error serializing diffs")
				continue
			}
			sndmsg = msg_format(TypeField, MsgAppRelease) +
				msg_format(UptField, string(sndmsgBytes))
			sectionAccess = false
			sectionAccessRequested = false
			display_d("Critical section released")

			// Request access to the critical section if the text has changed
		} else if (cur != lastText) && (!sectionAccessRequested) {
			sectionAccessRequested = true
			sndmsg = msg_format(TypeField, MsgAppRequest)
		}

		if sndmsg != "" {
			fmt.Println(sndmsg)
		}
		mutex.Unlock()
	}
}

func receive(textArea *widget.Entry, myWindow fyne.Window) {
	var rcvmsg string
	var rcvtyp string
	var rcvuptdiffs []utils.Diff

	reader := bufio.NewReader(os.Stdin)
	for {
		rcvmsgRaw, err := reader.ReadString('\n')
		if err != nil {
			//display_e("Error reading message : " + err.Error())
			continue
		}
		rcvmsg = strings.TrimSuffix(rcvmsgRaw, "\n")

		mutex.Lock()
		cur := textArea.Text
		rcvtyp = findval(rcvmsg, TypeField, true)
		if rcvtyp == "" {
			continue
		}

		switch rcvtyp {
		case MsgAppStartSc: // Receive start critical section message
			sectionAccess = true
			display_d("Critical section access granted")

		case MsgAppUpdate: // Receive update from remote version
			rcvupt := findval(rcvmsg, UptField, true)
			err := json.Unmarshal([]byte(rcvupt), &rcvuptdiffs)
			if err != nil {
				display_e("Error deserializing diffs")
				continue
			}

			oldTextUpdated := utils.ApplyDiffs(lastText, rcvuptdiffs) // Apply the diffs to the last remote text
			utils.SaveModifs(lastText, oldTextUpdated, localSaveFilePath)
			lastText = oldTextUpdated
			newText := utils.ApplyDiffs(cur, utils.ComputeDiffs(cur, lastText)) // Apply the diffs to the current text

			fyne.Do(func() {
				textArea.SetText(newText)
				textArea.Refresh()
			})

			display_d("Critical section updated")

		case MsgAppShallDie:
			var sndmsg = msg_format(TypeField, MsgAppDied)
			fmt.Println(sndmsg)
			display_w("App died")
			os.Stdout.Sync()
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				fyne.CurrentApp().Driver().Quit()
			}, true)
		}
		mutex.Unlock()
		rcvmsg = ""
	}
}

func saveTextToFile(path string, content string) error {

	return os.WriteFile(path, []byte(content), 0644)
}

func processText(oldContent string, newContent string) {

	utils.SaveModifs(oldContent, newContent, localSaveFilePath)
}

func initUI() (fyne.Window, *widget.Entry) {
	var content fyne.CanvasObject

	// Create app
	myApp := app.New()

	// Create a window
	myWindow := myApp.NewWindow("Distributed Editor n" + fmt.Sprint(*id))
	myWindow.Resize(fyne.NewSize(800, 600))

	// Define the text area
	textArea := widget.NewMultiLineEntry()
	textArea.SetPlaceHolder("Write something...")
	textArea.Wrapping = fyne.TextWrapWord

	// Load the saved text
	text, err := utils.GetUpdatedTextFromFile(0, "", localSaveFilePath)
	if err != nil {
		s_err := fmt.Sprintf("Error loading text from file: %v", err)
		display_e(s_err)
	}
	textArea.SetText(text)

	// Define a scrollable area containing the text area
	scrollable := container.NewScroll(textArea)
	scrollable.SetMinSize(fyne.NewSize(600, 400))

	// Set the window content
	if *debug {
		saveBtn := widget.NewButton("Save", func() {
			// Déclenche la sauvegarde via le channel
			go func() { saveTrigger <- struct{}{} }()
		})
		content = container.NewBorder(nil, saveBtn, nil, nil, scrollable)
	} else {
		content = container.NewBorder(nil, nil, nil, nil, scrollable)
	}

	myWindow.SetContent(content)

	// Set the window close intercept
	myWindow.SetCloseIntercept(func() {
		var sndmsg = msg_format(TypeField, MsgAppDied)
		fmt.Println(sndmsg)
		myWindow.Close()
	})

	return myWindow, textArea
}
