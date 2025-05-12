package main

import (
	"app/utils"
	"flag"
	"fmt"
	"log"
	"time"
	"os"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"sync"
	"strconv"
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

const outputDir string = "./output"

// Interval in seconds between autosaves
const autoSaveInterval = 1 * time.Second

var id *int = flag.Int("id", 0, "id of site")

var mutex = &sync.Mutex{}


func main() {

	// Parse command line arguments
	flag.Parse()
	createLogFile()

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
	var i int

	i = 0

	for {
		mutex.Lock()
		i = i + 1
		sndmsg = "message_" + strconv.Itoa(i) + "\n"
		fmt.Print(sndmsg)
		mutex.Unlock()
		time.Sleep(time.Duration(2) * time.Second)
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
		for i := 1; i < 6; i++ {
			l.Println("traitement message", i)
			time.Sleep(time.Duration(1) * time.Second)
		}
		mutex.Unlock()
		rcvmsg = ""
	}
}

func createLogFile() {
	
	// Create the output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Create the log file with the name "modifs_{id}.log"
	logFilePath := fmt.Sprintf("%s/modifs_%d.log", outputDir, *id)
	logFile, err := os.Create(logFilePath)
	if err != nil {
		log.Fatalf("Failed to create log file: %v", err)
	}
	logFile.Close()
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
    text, err := utils.GetUpdatedTextFromFile(0, "")
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

