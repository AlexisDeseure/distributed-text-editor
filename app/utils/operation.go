package Utils

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Operation représente une mise à jour complète du texte
// Stamp et VC pour l'ordre, ici VC est conservé mais non exploité
type Operation struct {
    SiteID int
    Stamp  int
    VC     []int
    Diff   string // "update:<full_text>"
}

// Site gère l'état, la concurrence et la communication
type Site struct {
    ID             int
    Lamport        int
    VC             []int
    Text           []rune
    LastWriteStamp int

    Log    map[string]Operation
    toSend []Operation
    In, Out chan []Operation

    mu       sync.Mutex
    OnUpdate func(fullText string)
}

// NewSite initialise un Site identifié par id et le nombre total de sites
func NewSite(id, totalSites int) *Site {
    return &Site{
        ID:      id,
        VC:      make([]int, totalSites),
        Log:     make(map[string]Operation),
        In:      make(chan []Operation, 10),
        Out:     make(chan []Operation, 10),
        toSend:  make([]Operation, 0),
    }
}

// cloneVC retourne une copie profonde de l'horloge vectorielle
func (s *Site) cloneVC() []int {
    vc := make([]int, len(s.VC))
    copy(vc, s.VC)
    return vc
}

// apply applique l'opération si elle est plus récente
func (s *Site) apply(op Operation) {
    if op.Stamp <= s.LastWriteStamp {
        return
    }
    const prefix = "update:"
    if strings.HasPrefix(op.Diff, prefix) {
        text := op.Diff[len(prefix):]
        s.Text = []rune(text)
        s.LastWriteStamp = op.Stamp
    }
}

// GenerateLocalOp crée une opération locale et la met en file d'envoi
func (s *Site) GenerateLocalOp(fullText string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    s.Lamport++
    s.VC[s.ID]++
    diff := "update:" + fullText
    op := Operation{SiteID: s.ID, Stamp: s.Lamport, VC: s.cloneVC(), Diff: diff}

    key := fmt.Sprintf("%d|%v", op.Stamp, op.VC)
    s.Log[key] = op
    s.apply(op)
    s.toSend = append(s.toSend, op)
}

// handleReceived stocke et applique les opérations reçues
func (s *Site) HandleReceived(ops []Operation) {
    s.mu.Lock()
    defer s.mu.Unlock()

    for _, op := range ops {
        key := fmt.Sprintf("%d|%v", op.Stamp, op.VC)
        if _, seen := s.Log[key]; seen {
            continue
        }
        s.Log[key] = op
        s.apply(op)
        s.toSend = append(s.toSend, op)
    }
}

// flushToOut envoie toutes les opérations en attente vers le canal Out
func (s *Site) FlushToOut() {
    s.mu.Lock()
    batch := s.toSend
    s.toSend = nil
    s.mu.Unlock()

    if len(batch) > 0 {
        s.Out <- batch
    }
}

// StartCommunication gère la réception et envoi périodique des opérations
func (s *Site) StartCommunication() {
    ticker := time.NewTicker(1000 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case ops := <-s.In:
            s.HandleReceived(ops)
            if s.OnUpdate != nil {
                s.OnUpdate(string(s.Text))
            }
        case <-ticker.C:
            s.FlushToOut()
        }
    }
}

// ConnectRing relie tous les sites en anneau (Out de i vers In de i+1 mod n)
func ConnectRing(sites []*Site) {
    n := len(sites)
    for i := 0; i < n; i++ {
        out := sites[i].Out
        in := sites[(i+1)%n].In
        go func(o chan []Operation, ii chan []Operation) {
            for ops := range o {
                ii <- ops
            }
        }(out, in)
    }
}