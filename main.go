package main

import (
	"encoding/json"
	// "fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
	"flag"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"sr05/Utils"
)

type VectorClock map[string]int

type MessageType string

const (
	MsgUpdate MessageType = "UPDATE"
	MsgToken  MessageType = "TOKEN"
)

// Message transporte uniquement l'horloge (et éventuellement un token)
type Message struct {
	Type   MessageType `json:"type"`
	Clock  VectorClock `json:"clock"`
	Holder string      `json:"holder,omitempty"` // utile pour MsgToken
}

// Instance gère le ring, envoie/recevra les horloges

type Instance struct {
	id        string
	clock     VectorClock
	prevPipe  io.ReadCloser
	nextPipe  io.WriteCloser

	receiveCh chan Message
	sendCh    chan Message

	// pour synchroniser lecture du log de diffs
	lastLine int
	text     string           // vue locale du texte
	lock     sync.Mutex
}

// NewInstance crée et lance les goroutines d'envoi/réception
func NewInstance(id string, prev io.ReadCloser, next io.WriteCloser) *Instance {
	inst := &Instance{
		id:        id,
		clock:     make(VectorClock),
		prevPipe:  prev,
		nextPipe:  next,
		receiveCh: make(chan Message, 10),
		sendCh:    make(chan Message, 10),
		lastLine:  1,
		text:      "",
	}
	go inst.receiveLoop()
	go inst.sendLoop()
	return inst
}

func (inst *Instance) receiveLoop() {
	dec := json.NewDecoder(inst.prevPipe)
	for {
		var msg Message
		if err := dec.Decode(&msg); err != nil {
			log.Println("receive error:", err)
			close(inst.receiveCh)
			return
		}
		inst.receiveCh <- msg
	}
}

func (inst *Instance) sendLoop() {
	enc := json.NewEncoder(inst.nextPipe)
	for msg := range inst.sendCh {
		if err := enc.Encode(msg); err != nil {
			log.Println("send error:", err)
			return
		}
	}
}

// mergeClocks met à jour inst.clock en prenant max(remote, local)
func mergeClocks(local VectorClock, remote VectorClock) {
	for id, t := range remote {
		if local[id] < t {
			local[id] = t
		}
	}
}

// File where the text will be saved
// const saveFilePath = "save.txt"

// Interval in seconds between autosaves
const autoSaveInterval = 1 * time.Second

// Write the text into the save file
// func saveTextToFile(path string, content string) error {

// 	return os.WriteFile(path, []byte(content), 0644)
// }

// Process the current text, and apply some operations
// The minimum is to save the text into the file
// Then the text can also be sent to another connected device
// This method is called periodically (see 'autoSaveInterval'), and one last time when the app closes
// It is never called if the text has not changed since last save
// func processText(oldContent string, newContent string) error {

// 	// Save the text and get a possible error
// 	err := saveTextToFile(saveFilePath, newContent)

// 	difftools.SaveModifs(oldContent, newContent)

// 	// Print the error
// 	if err != nil {

// 		fmt.Println("Error while saving:", err)

// 	// Notify that the text was successfully saved
// 	} else {

// 		fmt.Println("Autosave...")
// 	}

// 	return err
// }

func main() {

	id := flag.String("n", "0", "id site")
	inst := NewInstance(*id, os.Stdin, os.Stdout)

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
	text, err := difftools.GetUpdatedTextFromFile(0, "")
	if err != nil {
		log.Fatal(err)
	}
	textArea.SetText(text)

	// Define a scrollable area containing the text area
	scrollable := container.NewScroll(textArea)

	// Define the visible size of the scrollable area
	scrollable.SetMinSize(fyne.NewSize(600, 400))

	/*// Create a channel to communicate with the save goroutine
	// When the app closes, a boolean is passed to this channel to tell the goroutine to save one last time and stop
	stopAutoSave := make(chan bool)

	// Launch a goroutine to manage autosaving
	go func() {

		// Create a ticker (clock) so the goroutine knows when to perform a save
		ticker := time.NewTicker(autoSaveInterval)

		// Tell the goroutine to stop the clock when the app closes
		defer ticker.Stop()

		// Save the last saved version of the text, so that the goroutine knows when the text has changed
		lastSaved := textArea.Text

		// Infinite loop (ends when the goroutine returns)
		for {

			// Two processed cases
			select {

			// The interval time is up
			case <-ticker.C:

				// Get the current text from the text area
				current := textArea.Text

				// No need to save if the text has not changed
				if current != lastSaved {

					// Process the text
					// This includes saving it into a file, but eventually other actions could be performed
					processText(lastSaved, current)

					// Update the last saved version
					lastSaved = current
				}

			// The app is closing, a signal was sent to this goroutine
			case <-stopAutoSave:

				// Get the current text from the text area
				current := textArea.Text

				// No need to save if the text has not changed
				if current != lastSaved {

					// Process the text
					// This includes saving it into a file, but eventually other actions could be performed
					processText(lastSaved, current)
				}

				// Stop the goroutine
				return
			}
		}
	}()*/

	// Goroutine de réception et application des horloges
	go func() {
		for msg := range inst.receiveCh {
			switch msg.Type {
			case MsgUpdate:
				mergeClocks(inst.clock, msg.Clock)
				// lire et appliquer nouvelles modifs
				newText, err := difftools.GetUpdatedTextFromFile(inst.lastLine, inst.text)
				if (err != nil){
					log.Fatal(err)
				}
				inst.lock.Lock()
				inst.text = newText
				inst.lock.Unlock()
                // mettre à jour l'UI dans le thread principal Fyne
                textArea.SetText(newText)
                textArea.Refresh()
				inst.lastLine += difftools.LineCountSince(inst.lastLine)
			case MsgToken:
				// gestion du token...
			}
		}
	}()

	// Autosave + publication de l'horloge
	stop := make(chan struct{})
	go func() {
		tk := time.NewTicker(autoSaveInterval)
		defer tk.Stop()
		last := textArea.Text
		for {
			select {
			case <-tk.C:
				cur := textArea.Text
				if cur != last {
					difftools.SaveModifs(last, cur)
					// incrémenter l'horloge locale et envoyer seulement l'horloge
					inst.clock[inst.id]++
					inst.sendCh <- Message{Type: MsgUpdate, Clock: inst.clock}
					last = cur
				}
			case <-stop:
				cur := textArea.Text
				if cur != last {
					difftools.SaveModifs(last, cur)
					inst.clock[inst.id]++
					inst.sendCh <- Message{Type: MsgUpdate, Clock: inst.clock}
				}
				return
			}
		}
	}()

	// Set the window content
	myWindow.SetContent(container.NewBorder(nil, nil, nil, nil, scrollable))

	// Cleanup à la fermeture
	myWindow.SetCloseIntercept(func() {
		// arrêter autosave
		close(stop)

		// supprimer l'interception pour permettre la fermeture
		myWindow.SetCloseIntercept(nil)

		// enfin, fermer la fenêtre
		myWindow.Close()
	})

	// Display the window
	myWindow.ShowAndRun()
}
