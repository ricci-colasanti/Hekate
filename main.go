package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Message structure for communication
type Message struct {
	Type    string  `json:"type"`
	Value   float64 `json:"value,omitempty"`
	Running bool    `json:"running,omitempty"`
}

// Data point sent to webpage
type DataPoint struct {
	Timestamp  int64   `json:"timestamp"`
	Value      float64 `json:"value"`
	Multiplier float64 `json:"multiplier"`
}

// Server state
type ServerState struct {
	mu         sync.Mutex
	running    bool
	multiplier float64
	clients    map[*websocket.Conn]bool
}

var state = ServerState{
	running:    false,
	multiplier: 1.0,
	clients:    make(map[*websocket.Conn]bool),
}

func main() {
	http.HandleFunc("/", servePage)
	http.HandleFunc("/ws", handleWebSocket)

	go func() {
		fmt.Println("Server starting on http://localhost:8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	time.Sleep(500 * time.Millisecond)
	openBrowser("http://localhost:8080")

	select {} // Block forever
}

func servePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "hekate.html")
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Upgrade failed:", err)
		return
	}
	defer conn.Close()

	// Register client
	state.mu.Lock()
	state.clients[conn] = true
	state.mu.Unlock()

	log.Println("Client connected")

	// Send current state to new client
	sendState(conn)

	// Start data stream if running
	if state.running {
		go streamData(conn)
	}

	// Listen for messages from client
	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		handleClientMessage(conn, msg)
	}

	// Unregister client
	state.mu.Lock()
	delete(state.clients, conn)
	state.mu.Unlock()
	log.Println("Client disconnected")
}

func handleClientMessage(conn *websocket.Conn, msg Message) {
	state.mu.Lock()
	defer state.mu.Unlock()

	switch msg.Type {
	case "start":
		state.running = true
		log.Println("Started data stream")
		// Start streaming for all clients
		go broadcastData()

	case "stop":
		state.running = false
		log.Println("Stopped data stream")
		// Send stop to all clients
		broadcastMessage(Message{Type: "stopped"})

	case "multiplier":
		state.multiplier = msg.Value
		log.Printf("Multiplier set to: %.2f", state.multiplier)
		// Send new multiplier to all clients
		broadcastMessage(Message{
			Type:  "multiplier_updated",
			Value: state.multiplier,
		})
	}
}

func sendState(conn *websocket.Conn) {
	state.mu.Lock()
	defer state.mu.Unlock()

	conn.WriteJSON(Message{
		Type:    "state",
		Running: state.running,
		Value:   state.multiplier,
	})
}

func broadcastMessage(msg Message) {
	for conn := range state.clients {
		err := conn.WriteJSON(msg)
		if err != nil {
			log.Println("Broadcast error:", err)
			conn.Close()
			delete(state.clients, conn)
		}
	}
}

func broadcastData() {
	state.mu.Lock()
	running := state.running
	multiplier := state.multiplier
	state.mu.Unlock()

	if !running {
		return
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		state.mu.Lock()
		if !state.running {
			state.mu.Unlock()
			return
		}
		multiplier = state.multiplier
		state.mu.Unlock()

		// Generate data with multiplier
		baseValue := rand.Float64() * 100
		data := DataPoint{
			Timestamp:  time.Now().Unix(),
			Value:      baseValue * multiplier,
			Multiplier: multiplier,
		}

		jsonData, err := json.Marshal(data)
		if err != nil {
			log.Println("JSON marshal error:", err)
			continue
		}

		// Send to all clients
		state.mu.Lock()
		for conn := range state.clients {
			err := conn.WriteMessage(websocket.TextMessage, jsonData)
			if err != nil {
				log.Println("Write error:", err)
				conn.Close()
				delete(state.clients, conn)
			}
		}
		state.mu.Unlock()

		log.Printf("Sent: %.2f (multiplier: %.2f)", data.Value, data.Multiplier)
	}
}

func streamData(conn *websocket.Conn) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		state.mu.Lock()
		if !state.running {
			state.mu.Unlock()
			return
		}
		multiplier := state.multiplier
		state.mu.Unlock()

		data := DataPoint{
			Timestamp:  time.Now().Unix(),
			Value:      rand.Float64() * 100 * multiplier,
			Multiplier: multiplier,
		}

		err := conn.WriteJSON(data)
		if err != nil {
			log.Println("Stream error:", err)
			return
		}
	}
}

func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		log.Printf("Failed to open browser: %v", err)
		log.Printf("Please open %s manually", url)
	} else {
		log.Printf("Browser opened to %s", url)
	}
}
