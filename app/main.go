package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"app/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// message types
const (

	// message types to be sent to controler
	MsgAppRequest  string = "rqa" // request critical section
	MsgAppRelease  string = "rla" // release critical section
	MsgAppDied     string = "apd" // app died
	MsgCut         string = "cut" // save the vectorial clock value
	MsgInitialSize string = "siz" // number of lines in the log file
	MsgInitialText string = "txt" // Initial text when the app begins

	// message types to be receive from controler
	MsgAppStartSc    string = "ssa" // start critical section
	MsgAppUpdate     string = "upa" // update critical section
	MsgAppShallDie   string = "shd" // app shall die
	MsgReturnNewText string = "ret" // return the new common text content to the site

)

// message fields
const (
	TypeField               string = "typ" // type of message
	UptField                string = "upt" // content of update for application
	cutNumber               string = "cnb" // number of next cut
	NumberVirtualClockSaved string = "nbv" // number of virtual clock saved

)

var outputDir *string = flag.String("o", "./output", "output directory")

// Interval in seconds between autosaves
const autoSaveInterval = 500 * time.Millisecond

// const autoSaveInterval = 2 * time.Second

var id *int = flag.Int("id", 0, "id of site")

var debug *bool = flag.Bool("debug", false, "enable debug mode (manual save)")

var mutex = &sync.Mutex{}

var (
	localSaveFilePath string
	localCutFilePath  string = fmt.Sprintf("%s/cut.json", *outputDir)
)

var (
	lastText               string
	sectionAccess          bool = false
	sectionAccessRequested bool = false
)

var (
	cut  bool = false
	save bool = false
)

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

	unifyVersions(utils.LineCountSince(0, localSaveFilePath))

	go send(textArea)
	go receive(textArea)

	// Display the window
	myWindow.ShowAndRun()
}

// This function starts a cycle that gathers the most up-to-date version of the common text and propagates it to each site
func unifyVersions(size int) {
	sndmsg := msg_format(TypeField, MsgInitialSize) + msg_format(UptField, strconv.Itoa(size))
	fmt.Println(sndmsg)

	content, err := os.ReadFile(localSaveFilePath)
	if err != nil {
		display_e("Error while reading log file: " + err.Error())
		return
	}

	// \n cannot be sent to the standard output without being misinterpreted
	formatted := strings.ReplaceAll(string(content), "\n", "↩")
	sndmsg = msg_format(TypeField, MsgInitialText) + msg_format(UptField, string(formatted))
	fmt.Println(sndmsg)
}

// A goroutine to manage local text saving and and to send modifications to other sites via the controller
func send(textArea *widget.Entry) {
	var sndmsg string

	for {

		if !*debug {
			time.Sleep(autoSaveInterval)
		}

		sndmsg = ""
		mutex.Lock()
		cur := textArea.Text

		if cut {

			cut = false
			nextCutNumber, _ := GetNextCutNumber(localCutFilePath)
			sndmsg = msg_format(TypeField, MsgCut) +
				msg_format(cutNumber, nextCutNumber) +
				msg_format(NumberVirtualClockSaved, strconv.Itoa(0))

		} else if sectionAccess && (!*debug || save) {

			save = false
			// Check if the controller has granted access to the critical section
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
		} else if (cur != lastText) && (!sectionAccessRequested) && (!*debug || save) {
			sectionAccessRequested = true
			sndmsg = msg_format(TypeField, MsgAppRequest)
		}

		if sndmsg != "" {
			fmt.Println(sndmsg)
		}
		mutex.Unlock()
	}
}

// A goroutine to process received messages
func receive(textArea *widget.Entry) {
	var rcvmsg string
	var rcvtyp string
	var rcvuptdiffs []utils.Diff

	reader := bufio.NewReader(os.Stdin)

	for {

		rcvmsgRaw, err := reader.ReadString('\n')
		if err != nil {
			// display_e("Error reading message : " + err.Error())
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
			newText := utils.ApplyDiffs(cur, rcvuptdiffs) // Apply the diffs to the current text
			lastText = oldTextUpdated

			fyne.Do(func() {
				textArea.SetText(newText)
				textArea.Refresh()
			})

			display_d("Critical section updated")

		case MsgAppShallDie:

			sndmsg := msg_format(TypeField, MsgAppDied)
			fmt.Println(sndmsg)
			os.Stdout.Sync()
			fyne.CurrentApp().Driver().DoFromGoroutine(func() {
				fyne.CurrentApp().Driver().Quit()
			}, true)

		case MsgReturnNewText:

			text := findval(rcvmsg, UptField, false)
			original := strings.ReplaceAll(text, "↩", "\n")
			err := os.WriteFile(localSaveFilePath, []byte(original), 0o644)
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

	// button to create a cut and save vectorial clock
	cutBtn := widget.NewButton("Cut", func() {
		// triggers the saving of vector clocks
		mutex.Lock()
		defer mutex.Unlock()
		cut = true
	})

	// Set the window content
	if *debug {
		saveBtn := widget.NewButton("Save", func() {
			mutex.Lock()
			defer mutex.Unlock()
			save = true
		})
		bottomButtons := container.NewHBox(saveBtn, cutBtn)
		content = container.NewBorder(nil, bottomButtons, nil, nil, scrollable)
	} else {
		bottomButtons := container.NewHBox(cutBtn)
		content = container.NewBorder(nil, bottomButtons, nil, nil, scrollable)

	}

	myWindow.SetContent(content)

	// Set the window close intercept
	myWindow.SetCloseIntercept(func() {
		sndmsg := msg_format(TypeField, MsgAppDied)
		fmt.Println(sndmsg)
		myWindow.Close()
	})

	return myWindow, textArea
}
