package main

import (
	"app/utils"
	"flag"
	"fmt"
	"log"
	"os"
	"encoding/json"
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
const autoSaveInterval = 1 * time.Second

var id *int = flag.Int("id", 0, "id of site")

var mutex = &sync.Mutex{}

var localSaveFilePath string = fmt.Sprintf("%s/modifs_%d.log", outputDir, *id)

func main() {

	// Parse command line arguments
	flag.Parse()

	// Initialize the UI and get window and text area
	myWindow, textArea := initUI()

	go send(textArea)
	go receive(textArea)

	// Display the window
	myWindow.ShowAndRun()
}

func send(textArea *widget.Entry) {
	//TODO : Implement the send function with the message format to request the critical section, to send the release of critical section access and send update
	var sndmsg string
	last := textArea.Text
	for {
		cur := textArea.Text
		if cur != last {
			mutex.Lock()
			utils.SaveModifs(last, cur, localSaveFilePath)
			diffs := utils.ComputeDiffs(last, cur)
			sndmsgBytes, err := json.Marshal(diffs)
			if err != nil {
				log.Printf("Error serializing diffs: %v", err)
			} else {
				sndmsg = string(sndmsgBytes)
			}
			fmt.Println(sndmsg)
			mutex.Unlock()
			time.Sleep(autoSaveInterval)
			last = cur
		}
	}
}

func receive(textArea *widget.Entry) {
	//TODO : Implement the receive function with the message format to receive the critical section access and updates from remote received
	var rcvmsg string
	l := log.New(os.Stderr, "", 0)

	for {
		fmt.Scanln(&rcvmsg)
		mutex.Lock()
		l.Println("reception <", rcvmsg, ">")
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
		log.Fatal(err)
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
