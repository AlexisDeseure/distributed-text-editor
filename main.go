package main

import (

	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

)

// File where the text will be saved
const saveFilePath = "save.txt"

// Interval in seconds between autosaves
const autoSaveInterval = 1 * time.Second


// Load the text from the save file
func loadTextFromFile(path string) string {

	data, err := os.ReadFile(path)

	// There was an error
	if err != nil {

		// The file does not exist yet, so the method returns an empty string
		return ""
	}

	return string(data)
}


// Write the text into the save file
func saveTextToFile(path string, content string) error {

	return os.WriteFile(path, []byte(content), 0644)
}


// Process the current text, and apply some operations
// The minimum is to save the text into the file
// Then the text can also be sent to another connected device
// This method is called periodically (see 'autoSaveInterval'), and one last time when the app closes
// It is never called if the text has not changed since last save
func processText(content string) error {

	// Save the text and get a possible error
	err := saveTextToFile(saveFilePath, content)

	// Print the error
	if err != nil {

		fmt.Println("Error while saving:", err)

	// Notify that the text was successfully saved
	} else {

		fmt.Println("Autosave...")
	}

	return err
}


func main() {

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
	textArea.SetText(loadTextFromFile(saveFilePath))

	// Define a scrollable area containing the text area
	scrollable := container.NewScroll(textArea)

	// Define the visible size of the scrollable area
	scrollable.SetMinSize(fyne.NewSize(600, 400))

	// Create a channel to communicate with the save goroutine
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
					processText(current)

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
					processText(current)
				}

				// Stop the goroutine
				return
			}
		}
	}()

	// Cleanup when the app closes
	myWindow.SetCloseIntercept(func() {

		// Signal the autosave goroutine to stop
		stopAutoSave <- true

		// Quit the app
		myApp.Quit()
	})

	// Set the window content
	myWindow.SetContent(container.NewBorder(nil, nil, nil, nil, scrollable))

	// Display the window
	myWindow.ShowAndRun()
}