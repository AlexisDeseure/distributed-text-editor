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


func main() {

	// Parse command line arguments
	flag.Parse()

	createLogFile()

	// Create the app
	myApp := app.New()

	// Create a window
	myWindow := myApp.NewWindow("SR05 Editor")

	// Set window size
	myWindow.Resize(fyne.NewSize(800, 600))

	// Define the text area
	textArea := widget.NewMultiLineEntry()
	textArea.SetPlaceHolder("Write something...")

	// Set the wrapping so that the texts gets to the next line when it is wider than the text area
	textArea.Wrapping = fyne.TextWrapWord

	// Load the saved text
	//textArea.SetText(loadTextFromFile(saveFilePath))
	text, err := Utils.GetUpdatedTextFromFile(0, "")
	if err != nil {
		log.Fatal(err)
	}
	textArea.SetText(text)

	// Define a scrollable area containing the text area
	scrollable := container.NewScroll(textArea)

	// Define the visible size of the scrollable area
	scrollable.SetMinSize(fyne.NewSize(600, 400))

	// // Goroutine de réception et application des horloges
	// go func() {
	// 	for msg := range inst.receiveCh {
	// 		switch msg.Type {
	// 		case MsgUpdate:
	// 			// lire et appliquer nouvelles modifs
	// 			newText, err := Utils.GetUpdatedTextFromFile(inst.lastLine, inst.text)
	// 			if err != nil {
	// 				log.Fatal(err)
	// 			}
	// 			inst.lock.Lock()
	// 			inst.text = newText
	// 			inst.lock.Unlock()
	// 			// mettre à jour l'UI dans le thread principal Fyne
	// 			textArea.SetText(newText)
	// 			textArea.Refresh()
	// 			inst.lastLine += Utils.LineCountSince(inst.lastLine)
	// 		case MsgToken:
	// 			// gestion du token...
	// 		}
	// 	}
	// }()


	// Set the window content
	myWindow.SetContent(container.NewBorder(nil, nil, nil, nil, scrollable))

	// Cleanup à la fermeture
	myWindow.SetCloseIntercept(func() {

		// supprimer l'interception pour permettre la fermeture
		myWindow.SetCloseIntercept(nil)

		// enfin, fermer la fenêtre
		myWindow.Close()
	})

	// Display the window
	myWindow.ShowAndRun()
}