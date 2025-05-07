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

// Interval in seconds between autosaves
const autoSaveInterval = 1 * time.Second


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
