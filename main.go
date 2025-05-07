package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"sr05/Utils"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type VectorClock map[int]int

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
	id       int
	clock    VectorClock
	prevPipe io.ReadCloser
	nextPipe io.WriteCloser

	receiveCh chan Message
	sendCh    chan Message

	// pour synchroniser lecture du log de diffs
	lastLine int
	text     string // vue locale du texte
	lock     sync.Mutex
}

// NewInstance crée et lance les goroutines d'envoi/réception
func NewInstance(id int, prev io.ReadCloser, next io.WriteCloser) *Instance {
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

/*func main() {

	id := flag.Int("n", 0, "id site")
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
	text, err := Utils.GetUpdatedTextFromFile(0, "")
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
				newText, err := Utils.GetUpdatedTextFromFile(inst.lastLine, inst.text)
				if err != nil {
					log.Fatal(err)
				}
				inst.lock.Lock()
				inst.text = newText
				inst.lock.Unlock()
				// mettre à jour l'UI dans le thread principal Fyne
				textArea.SetText(newText)
				textArea.Refresh()
				inst.lastLine += Utils.LineCountSince(inst.lastLine)
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
					Utils.SaveModifs(last, cur)
					// incrémenter l'horloge locale et envoyer seulement l'horloge
					inst.clock[inst.id]++
					inst.sendCh <- Message{Type: MsgUpdate, Clock: inst.clock}
					last = cur
				}
			case <-stop:
				cur := textArea.Text
				if cur != last {
					Utils.SaveModifs(last, cur)
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
}*/

// main crée 3 sites, les relie et ouvre 3 fenêtres Fyne
func main() {
    total := 3
    flag.Parse()

    // Initialisation des sites
    sites := make([]*Utils.Site, total)
    for i := 0; i < total; i++ {
        sites[i] = Utils.NewSite(i, total)
    }

    // Liaison en anneau
    Utils.ConnectRing(sites)

    // Démarrage de la communication pour chaque site
    for _, s := range sites {
        go s.StartCommunication()
    }

    // GUI Fyne
    app := app.New()
    for _, site := range sites {
        s := site
        win := app.NewWindow(fmt.Sprintf("SR05 Editor %d", s.ID))
        win.Resize(fyne.NewSize(600, 400))

        textArea := widget.NewMultiLineEntry()
        textArea.Wrapping = fyne.TextWrapWord
        textArea.SetText(string(s.Text))

        scroll := container.NewScroll(textArea)
        win.SetContent(scroll)

        // Callback pour mise à jour UI
        s.OnUpdate = func(fullText string) {
            fyne.Do(func() {
                textArea.SetText(fullText)
                textArea.Refresh()
            })
        }

        // Propagation immédiate à chaque modification
        textArea.OnChanged = func(cur string) {
            s.GenerateLocalOp(cur)
            s.FlushToOut()
        }

        win.Show()
    }
    app.Run()
}