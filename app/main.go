package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"app/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// message types
const (
	// message types to be sent to controler
	MsgAppRequest string = "rqa" // request critical section
	MsgAppRelease string = "rla" // release critical section
	MsgAppDied    string = "apd" // notify the controller that the app has been closed

	// message types to be receive from controler
	MsgAppStartSc        string = "ssa"  // start critical section
	MsgAppUpdate         string = "upa"  // update critical section
	MsgReturnInitialText string = "ret"  // return the initial common text content to the site
	MsgReturnText        string = "ret2" // give the current text content to the site
)

// message fields
const (
	TypeField               string = "typ" // type of message
	UptField                string = "upt" // content of update for application
	SiteIdField             string = "sid" // site id of sender
)

var outputDir *string = flag.String("o", "./output", "output directory")

// Interval in seconds between autosaves
const autoSaveInterval = 500 * time.Millisecond

// const autoSaveInterval = 2 * time.Second

var filename *string = flag.String("f", "New document", "name of the file to edit")
var id *string = flag.String("id", "0", "id of site")

var mutex = &sync.Mutex{}

var (
	localSaveFilePath string                                          //path to the local save of the shared file in a log format
)

var (
	lastText               string         //contains the last local text sync with the shared version
	sectionAccess          bool   = false // true if the app have access to critical section
	sectionAccessRequested bool   = false // true if app has request to access the critical section
)

var (
	// Channel to signal goroutines to stop
	stopChan = make(chan struct{})
)

func main() {
	// Parse command line arguments
	flag.Parse()
	display_d("Starting app with id: " + *id)
	// Sanitize filename by replacing spaces and special characters with "_"
	reg := regexp.MustCompile("[^a-zA-Z0-9_-]+")
	sanitizedFilename := reg.ReplaceAllString(*filename, "_")
	// However, for simplicity with current dependencies, we'll stick to ReplaceAll for now.
	// If more complex sanitization is needed, the regexp approach is recommended.

	localSaveFilePath = fmt.Sprintf("%s/%s.log", *outputDir, sanitizedFilename)

	// Initialize the UI and get window and text area
	myWindow, textArea := initUI()
	lastText = textArea.Text

	// Launch the initialization process to be sure each site has the same initial local save
	unifyVersions(textArea)

	// Start send/receive routines
	go send(textArea)
	go receive(textArea)

	// Display the windownewSiteKnown
	myWindow.ShowAndRun()
}

// This function starts a cycle that gathers the most up-to-date version of the common text and propagates it to each site
func unifyVersions(textArea *widget.Entry) {
	reader := bufio.NewReader(os.Stdin)

	for {
		display_d("Waiting for initial message from controller...")
		rcvmsgRaw, err := reader.ReadString('\n')
		if err != nil {
			// display_e("Error reading message : " + err.Error())
			continue
		}
		// delete last "\n"
		rcvmsg := strings.TrimSuffix(rcvmsgRaw, "\n")
		rcvtyp := findval(rcvmsg, TypeField, true)
		if rcvtyp == "" {
			continue
		}
		if rcvtyp == MsgReturnInitialText { // Receive a new text message : corresponds to the initial text sent by the controller

			text := findval(rcvmsg, UptField, false)
			idrcv := findval(rcvmsg, SiteIdField, false)
			if idrcv == *id { // If the text is not empty, we are a secondary site so we need to update the local save file
				// with the text received from the controller
				display_d("Received initial message from controller, updating local save file as we are a secondary site")
				original := strings.ReplaceAll(text, "↩", "\n")
				// Erase the local save with the one received
				err := os.WriteFile(localSaveFilePath, []byte(original), 0o644)
				if err != nil {
					display_e("Error while writing into log file: " + err.Error())
				}

				// Get the new content and replace the loacal save var + refresh UI
				content, err := utils.GetUpdatedTextFromFile(0, "", localSaveFilePath)
				if err != nil {
					display_e("Error while reading log file: " + err.Error())
				}
				fyne.Do(func() {
					textArea.SetText(content)
					textArea.Refresh()
					lastText = content
				})
			} else { // If the text is empty, we are the first site so we can keep the current text area content
				display_d("Received initial message from controller, no need to update local save file as we are the first site")
			}
			return // Exit the loop after receiving the initial text to go to the main loop and display the UI
		}
	}
}

// A goroutine to manage local text saving and and to send modifications to other sites via the controller
func send(textArea *widget.Entry) {
	var sndmsg string

	for {
		time.Sleep(autoSaveInterval) // Wait for the next autosave interval

		sndmsg = ""

		mutex.Lock()
		cur := textArea.Text // current text displayed on the Fyne UI

		if sectionAccess {
			// if the controller has granted access to the critical section

			// local save can be updated with user modifications
			newTextDiffs := utils.ComputeDiffs(lastText, cur)
			newText := utils.ApplyDiffs(lastText, newTextDiffs)
			utils.SaveModifs(lastText, newText, localSaveFilePath)
			lastText = newText

			// app can release critical section access with its modifications
			sndmsgBytes, err := json.Marshal(newTextDiffs)
			if err != nil {
				display_e("Error serializing diffs")
				continue
			}

			// share the new text content with the controller in case of new site to be inserted
			formattedText := getCurrentTextContentFormated()
			sndNewTextFormated := msg_format(TypeField, MsgReturnText) +
				msg_format(UptField, formattedText) +
				msg_format(SiteIdField, "-1") // -1 means that the demand is not cibled to a specific site and can engender multiple new connections
			fmt.Println(sndNewTextFormated)

			// send the critical section release message
			sndmsg = msg_format(TypeField, MsgAppRelease) +
				msg_format(UptField, string(sndmsgBytes))

			//booleans reseted to false
			sectionAccess = false
			sectionAccessRequested = false
			display_d("Critical section released")

		} else if (cur != lastText) && (!sectionAccessRequested) {
			// Request access to the critical section if the text has changed
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
		// delete last "\n"
		rcvmsg = strings.TrimSuffix(rcvmsgRaw, "\n")

		mutex.Lock()

		cur := textArea.Text // current text displayed on the Fyne UI
		rcvtyp = findval(rcvmsg, TypeField, true)
		if rcvtyp == "" {
			continue
		}

		switch rcvtyp {

		case MsgAppDied:
			close(stopChan) // Signal to stop the application

		case MsgReturnText: // Demand to return the current text content
			senderId := findval(rcvmsg, SiteIdField, true)
			formatted := getCurrentTextContentFormated()
			sndmsg := msg_format(TypeField, MsgReturnText) +
				msg_format(UptField, formatted) +
				msg_format(SiteIdField, senderId)
			fmt.Println(sndmsg) // send the content
			display_d("Returning current shared local text content to controller")

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

			// Apply the modifs on the local copy of the shared file without considering local unsaved user modifications
			oldTextUpdated := utils.ApplyDiffs(lastText, rcvuptdiffs) // Apply the diffs to the last remote text
			utils.SaveModifs(lastText, oldTextUpdated, localSaveFilePath)
			// Apply the modifs receive on the UI considering the local unsaved user modifications
			newText := utils.ApplyDiffs(cur, rcvuptdiffs) // Apply the diffs to the current text
			// Update the shared file copy without unsaved local user modifs
			lastText = oldTextUpdated

			// Refresh UI
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

// A function to initialize the UI
func initUI() (fyne.Window, *widget.Entry) {
	var content fyne.CanvasObject

	// Create the app with forced light theme
	myApp := app.New()
	myApp.Settings().SetTheme(&CustomTheme{})

	// Create the window
	myWindow := myApp.NewWindow(*filename)
	myWindow.Resize(fyne.NewSize(800, 600))

	// Create the text area
	textArea := widget.NewMultiLineEntry()
	textArea.SetPlaceHolder("Write something...")
	textArea.Wrapping = fyne.TextWrapWord

	// Create a white background behind the text area
	whiteBackground := canvas.NewRectangle(color.White)
	whiteBackground.Resize(fyne.NewSize(800, 600)) // ensure it covers

	// Stack the white background and the text area
	textContainer := container.NewStack(whiteBackground, textArea)

	// Load the saved text
	text, err := utils.GetUpdatedTextFromFile(0, "", localSaveFilePath)
	if err != nil {
		s_err := fmt.Sprintf("Error loading text from file: %v", err)
		display_e(s_err)
	}
	textArea.SetText(text)

	// Scrollable area
	scrollable := container.NewScroll(textContainer)
	scrollable.SetMinSize(fyne.NewSize(600, 400))

	// Bottom of window
	content = container.NewBorder(nil, nil, nil, nil, scrollable)

	// Set the content
	myWindow.SetContent(content)
	// Capture window close
	myWindow.SetCloseIntercept(func() {
		// Change the content of the main window to show closing message
		message := widget.NewLabel("Application closing...\nPlease wait.")
		message.Alignment = fyne.TextAlignCenter

		// Center the message in the main window
		closingContent := container.NewCenter(message)
		myWindow.SetContent(closingContent)

		// Send message to controller and clean shutdown
		go func() {
			display_w("Application closed by user, sending message to controller")
			sndmsg := msg_format(TypeField, MsgAppDied)
			fmt.Println(sndmsg)

			// wait to receive the confirmation from the controller to close the window
			<-stopChan

			// Close the window properly
			fyne.Do(func() {
				myWindow.Close()
			})
			os.Exit(0) // Exit the application
		}()
	})

	return myWindow, textArea
}

type CustomTheme struct{}

func (m *CustomTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground, theme.ColorNameInputBackground:
		return color.White
	case theme.ColorNameButton, theme.ColorNameDisabledButton:
		return color.White
	case theme.ColorNameForeground, theme.ColorNamePrimary:
		return color.Black
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

func (m *CustomTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m *CustomTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m *CustomTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 24 // Bigger font size
	default:
		return theme.DefaultTheme().Size(name)
	}
}
