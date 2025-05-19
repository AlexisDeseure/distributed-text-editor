package main

import (
	"app/utils"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"
	"bufio"
	"strings"
	"strconv"

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
	MsgAppStartSc  string = "ssa" // start critical section
	MsgAppUpdate   string = "upa" // update critical section
	MsgInitialSize string = "siz" // number of lines in the log file
	MsgInitialText string = "txt" // Initial text when the app begins
	MsgReturnNewText string = "ret" // return the new common text content to the site 
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

	sendInitial(utils.LineCountSince(0, localSaveFilePath))

	go send(textArea)
	go receive(textArea, myWindow)

	// Display the window
	myWindow.ShowAndRun()
}

func sendInitial(size int) {
		//if *id != 0 {return}
		sndmsg := msg_format(TypeField, MsgInitialSize) +
				msg_format(UptField, strconv.Itoa(size))
		//display_d(sndmsg)
		fmt.Println(sndmsg)

		content, err := os.ReadFile(localSaveFilePath)
		if err != nil {
			display_e("Error while reading log file: " + err.Error())
			return
		}
		formatted := strings.ReplaceAll(string(content), "\n", "↩")
		sndmsg = msg_format(TypeField, MsgInitialText) + msg_format(UptField, string(formatted))
		display_d(sndmsg)
		fmt.Println(sndmsg)
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
			display_e("Error reading message : " + err.Error())
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

		case MsgReturnNewText:

			text := findval(rcvmsg, UptField, false)
			original := strings.ReplaceAll(text, "↩", "\n")
			err := os.WriteFile(localSaveFilePath, []byte(original), 0644)
			if err != nil {
				display_e("Error while writing into log file: " + err.Error())
			}

			content, err := utils.GetUpdatedTextFromFile(0, "", localSaveFilePath)
			if err != nil {
				display_e("Error while reading log file: " + err.Error())
			}

			fyne.Do(func() {
				textArea.SetText(content)
				textArea.Refresh()
				lastText = content
			})
		}
		mutex.Unlock()
		rcvmsg = ""
	}
}

func initUI() (fyne.Window, *widget.Entry) {
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
	myWindow.SetContent(container.NewBorder(nil, nil, nil, nil, scrollable))

	// Set the window close intercept
	myWindow.SetCloseIntercept(func() {
		var sndmsg = msg_format(TypeField, MsgAppDied)
		fmt.Println(sndmsg)
		myWindow.Close()
	})

	return myWindow, textArea
}
