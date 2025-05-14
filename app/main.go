package main

import (
	"app/utils"
	"encoding/json"
	"flag"
	"fmt"
	"os"
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
	// message type to be receive from controler
	MsgAppStartSc string = "ssa" // start critical section
	MsgAppUpdate  string = "upa" // update critical section
)

const (
	TypeField string = "typ" // type of message
	UptField  string = "upt" // content of update for application
)

const outputDir string = "../output"

// Interval in seconds between autosaves
const autoSaveInterval = 3 * time.Second

var id *int = flag.Int("id", 0, "id of site")

var mutex = &sync.Mutex{}

var localSaveFilePath string = fmt.Sprintf("%s/modifs_%d.log", outputDir, *id)

var lastText string
var sectionAccess bool = false
var sectionAccessRequested bool = false

func main() {

	// Parse command line arguments
	flag.Parse()

	// Initialize the UI and get window and text area
	myWindow, textArea := initUI()

	lastText = textArea.Text

	go send(textArea)
	go receive(textArea)

	// Display the window
	myWindow.ShowAndRun()
}

func send(textArea *widget.Entry) {
	var sndmsg string
	for {
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
		time.Sleep(autoSaveInterval)
	}
}

func receive(textArea *widget.Entry) {
	var rcvmsg string
	var rcvtyp string
	var rcvuptdiffs []utils.Diff

	for {
		fmt.Scanln(&rcvmsg)
		mutex.Lock()
		cur := textArea.Text
		rcvtyp = findval(rcvmsg, TypeField, true)
		if rcvtyp == "" {
			continue
		}

		switch rcvtyp {
		case MsgAppStartSc: // Receive start critical section message
			sectionAccess = true
			sectionAccessRequested = false
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
	// Create app
	myApp := app.New()

	// Create a window
	myWindow := myApp.NewWindow("SR05 Editor")
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
	myWindow.SetContent(container.NewBorder(nil, nil, nil, nil, scrollable))

	// Set the window close intercept
	myWindow.SetCloseIntercept(func() {
		myWindow.SetCloseIntercept(nil)
		myWindow.Close()
	})

	return myWindow, textArea
}
