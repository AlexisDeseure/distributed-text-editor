package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	fieldsep  = "~"
	keyvalsep = "`"
)

type CompareElement struct {
	Clock int
	Id    string
}

type StateObject struct {
	Type  string
	Clock int
}

type StateMap map[string]*StateObject

// msg_format constructs a key-value string using predefined separators
func msg_format(key string, val string) string {
	return fieldsep + keyvalsep + key + keyvalsep + val
}

// resetStamp returns the next logical timestamp, ensuring monotonicity
func resetStamp(stamp, stamprcv int) int {
	if stamp < stamprcv {
		return stamprcv + 1
	}
	return stamp + 1
}

// findval searches a formatted message for a given key and returns its value
func findval(msg string, key string, verbose bool) string {
	if len(msg) < 4 {
		if verbose {
			display_e("Message length too short: " + msg)
		}
		return ""
	}

	sep := msg[0:1]
	tab_allkeyvals := strings.Split(msg[1:], sep)

	for _, keyval := range tab_allkeyvals {

		if len(keyval) < 4 {
			continue
		}

		equ := keyval[0:1]
		tabkeyval := strings.Split(keyval[1:], equ)
		if tabkeyval[0] == key {
			return tabkeyval[1]
		}
	}
	if verbose {
		err_msg := fmt.Sprintf("Key %s not found in message", key)
		display_w(err_msg)
	}
	return ""
}

// updateVectorialClock merges two vector clocks and increments the local entry
//
//	func updateVectorialClock(oldVectorialClock []int, newVectorialClock []int) []int {
//		for i := range oldVectorialClock {
//			oldVectorialClock[i] = int(math.Max(float64(oldVectorialClock[i]), float64(newVectorialClock[i])))
//		}
//		oldVectorialClock[*id]++
//		return oldVectorialClock
//	}
func updateVectorialClock(localClock map[string]int, receivedClock map[string]int, mySiteID string) map[string]int {
	for siteID, receivedValue := range receivedClock {
		if localValue, exists := localClock[siteID]; !exists || receivedValue > localValue {
			localClock[siteID] = receivedValue
		}
	}
	// Incrément du compteur local
	localClock[mySiteID]++
	return localClock
}

// CreateDefaultTab initializes a Tab of length n with default type and zero clock
func CreateDefaultStateMap(siteID string) StateMap {
	// arr := make(Tab, n)
	// for i := range arr {
	// 	arr[i] = TabElement{Type: MsgReleaseSc, Clock: 0}
	// }
	// return arr
	stateMap := make(map[string]*StateObject)
	stateMap[siteID] = &StateObject{Type: MsgReleaseSc, Clock: 0}
	return stateMap
}

func AddSiteToStateMap(stateMap *StateMap, siteID string) {
	if _, exists := (*stateMap)[siteID]; !exists {
		(*stateMap)[siteID] = &StateObject{
			Type:  MsgReleaseSc,
			Clock: -1, // Initialize with -1 to indicate not set : to be sure we won't get the priority
		}
	}
}

// CreateTabInit returns an integer slice of length n filled with -1
func CreateTabInit() map[string]int {
	return make(map[string]int)
}

// timestampComparison returns true if element a precedes b by clock, then id
func timestampComparison(a, b CompareElement) bool {
	if a.Clock < b.Clock {
		return true
	} else if a.Clock == b.Clock && a.Id < b.Id {
		return true
	}
	return false
}

// verifyScApproval checks if the local site can enter the critical section and signals approval
func verifyScApproval(tab StateMap, myID string) {
	var sndmsg string
	if tab[myID].Type == MsgRequestSc {

		site_elem := CompareElement{Clock: tab[myID].Clock, Id: myID}

		for i, el := range tab {
			inter_elem := CompareElement{Clock: el.Clock, Id: i}
			if i != myID && !timestampComparison(site_elem, inter_elem) {
				return
			}
		}

		sndmsg = msg_format(TypeField, MsgAppStartSc)
		fmt.Println(sndmsg)
		display_d("Entering critical section")
	}
}

// // saveCutJson records a vectorial clock under a given cut and action in a JSON file
// func saveCutJson(cutNumber string, filePath string, newcontent string) error {
// 	fichier, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0o644)
// 	if err != nil {
// 		return fmt.Errorf("error opening/creating file: %w", err)
// 	}
// 	defer fichier.Close()

// 	contenu, err := io.ReadAll(fichier)
// 	if err != nil {
// 		return fmt.Errorf("error reading file: %w", err)
// 	}

// 	// convert the file content into json structure
// 	var data map[string]interface{}
// 	if len(contenu) == 0 {
// 		data = make(map[string]interface{})
// 	} else {
// 		err = json.Unmarshal(contenu, &data)
// 		if err != nil {
// 			return fmt.Errorf("error while parsing JSON: %w", err)
// 		}
// 	}

// 	// json structure: {cutNumber: {siteActionNumber: vectorialClock}}
// 	data["cutNumber"] = newcontent

// 	_, err = fichier.Seek(0, 0)
// 	if err != nil {
// 		return fmt.Errorf("error seeking file start: %w", err)
// 	}

// 	err = fichier.Truncate(0)
// 	if err != nil {
// 		return fmt.Errorf("error truncating file: %w", err)
// 	}

// 	modifiedContent, err := json.MarshalIndent(data, "", "  ")
// 	if err != nil {
// 		return fmt.Errorf("error marshalling JSON: %w", err)
// 	}

// 	_, err = fichier.Write(modifiedContent)
// 	if err != nil {
// 		return fmt.Errorf("error writing to file: %w", err)
// 	}

// 	return nil
// }

func saveCutJson(cutNumber string, filePath string, newcontent string) error {
	fichier, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("error opening/creating file: %w", err)
	}
	defer fichier.Close()

	contenu, err := io.ReadAll(fichier)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	var data map[string]interface{}
	if len(contenu) == 0 {
		data = make(map[string]interface{})
	} else {
		err = json.Unmarshal(contenu, &data)
		if err != nil {
			return fmt.Errorf("error while parsing JSON from file: %w", err)
		}
	}

	// --- DÉBUT DE LA MODIFICATION ---

	// 1. Désérialiser la chaîne 'newcontent' qui est elle-même du JSON.
	//    On utilise interface{} car on ne connaît pas la structure exacte à l'avance,
	//    ce qui la rend très flexible.
	var parsedNewContent interface{}
	err = json.Unmarshal([]byte(newcontent), &parsedNewContent)
	if err != nil {
		// Si 'newcontent' n'est pas une chaîne JSON valide, on retourne une erreur.
		return fmt.Errorf("error parsing newcontent string as JSON: %w", err)
	}

	// 2. Assigner l'objet Go (et non la chaîne) à la clé correspondante.
	//    On utilise la variable `cutNumber` comme clé, ce qui semble être l'intention.
	data[cutNumber] = parsedNewContent

	// --- FIN DE LA MODIFICATION ---

	_, err = fichier.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("error seeking file start: %w", err)
	}

	err = fichier.Truncate(0)
	if err != nil {
		return fmt.Errorf("error truncating file: %w", err)
	}

	// json.MarshalIndent va maintenant fonctionner correctement car `data`
	// contient des structures Go natives (maps, slices, etc.) et non des chaînes pré-formatées.
	modifiedContent, err := json.MarshalIndent(data, "", "  ") // "  " pour une belle indentation
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %w", err)
	}

	_, err = fichier.Write(modifiedContent)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	return nil
}

func GetNextCutNumber(filePath string) (string, error) {
	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return "cut_number_1", fmt.Errorf("failed to read file: %w", err)
	}

	var cuts map[string]interface{} // Use interface{} as values might not strictly be strings
	err = json.Unmarshal(data, &cuts)
	if err != nil {
		return "cut_number_1", fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	maxCut := -1
	for key := range cuts {
		if !strings.HasPrefix(key, "cut_number_") {
			continue
		}

		parts := strings.Split(key, "_")
		if len(parts) != 3 {
			continue
		}

		numStr := parts[2]
		num, err := strconv.Atoi(numStr)
		if err != nil {
			// Ignore keys where the number part is not a valid integer
			continue
		}

		if num > maxCut {
			maxCut = num
		}
	}

	nextCut := maxCut + 1
	return fmt.Sprintf("cut_number_%d", nextCut), nil
}

func FormatJsonCutData(vectorialClock map[string]int, textContent string) (string, error) {
	var cutValue CutJsonValue = CutJsonValue{
		VectorialClock: vectorialClock,
		TextContent:    textContent,
	}

	// jsonData, err := json.MarshalIndent(cutValue, "", "  ")
	jsonData, err := json.Marshal(cutValue)
	if err != nil {
		log.Fatal(err)
	}
	// wrappedJson := map[string]string{
	// 	siteActionNumber: string(jsonData),
	// }
	// finalJsonData, err := json.MarshalIndent(wrappedJson, "", "  ")
	// if err != nil {
	// 	return "", fmt.Errorf("EUFEZUIFGEGZIFGZEerror marshalling wrapped JSON: %w", err)
	// }
	display_e("RENDU DU JSON FORMAT" + string(jsonData))
	return string(jsonData), nil
}

func MergeJsonStrings(jsoncontent, textcontent string, newkey string) (string, error) {
	var mapcontent map[string]interface{}

	// Déserialiser les deux chaînes en maps
	if err := json.Unmarshal([]byte(jsoncontent), &mapcontent); err != nil {
		return "", fmt.Errorf("erreur de parsing du premier JSON: %w", err)
	}

	mapcontent[newkey] = textcontent
	// Convertir le JSON fusionné en string
	mergedJsonBytes, err := json.MarshalIndent(mapcontent, "", "  ")
	if err != nil {
		return "", fmt.Errorf("erreur de re-conversion en JSON: %w", err)
	}

	return string(mergedJsonBytes), nil
}
