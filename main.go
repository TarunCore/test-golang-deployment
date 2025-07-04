package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan string)
var messages []string
var mu sync.Mutex

func main() {
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", handleConnections)

	go handleMessages()

	fmt.Println("Server started at http://localhost:8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	tmpl := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Go Chat App</title>
		<style>
			body {
				font-family: Arial, sans-serif;
				background: #f9f9f9;
				display: flex;
				justify-content: center;
				align-items: center;
				height: 100vh;
			}
			.container {
				width: 400px;
				background: #fff;
				border: 1px solid #ddd;
				padding: 20px;
				box-shadow: 0 0 10px rgba(0,0,0,0.1);
				border-radius: 8px;
			}
			h2 {
				text-align: center;
				color: #333;
			}
			#chat {
				height: 300px;
				overflow-y: auto;
				border: 1px solid #ccc;
				background: #f0f0f0;
				padding: 10px;
				margin-bottom: 10px;
				border-radius: 4px;
			}
			#msg {
				width: 75%;
				padding: 8px;
				border: 1px solid #ccc;
				border-radius: 4px;
			}
			button {
				padding: 8px 12px;
				border: none;
				background: #007bff;
				color: white;
				border-radius: 4px;
				cursor: pointer;
			}
			button:hover {
				background: #0056b3;
			}
		</style>
	</head>
	<body>
		<div class="container">
			<h2>Go Chat Room</h2>
			<div id="chat"></div>
			<input id="msg" placeholder="Enter message..." />
			<button onclick="send()">Send</button>
		</div>

		<script>
			let socket = new WebSocket("wss://" + location.host + "/ws");
			let chat = document.getElementById("chat");

			socket.onmessage = function(e) {
				let p = document.createElement("div");
				p.textContent = e.data;
				chat.appendChild(p);
				chat.scrollTop = chat.scrollHeight;
			};

			function send() {
				let input = document.getElementById("msg");
				if (input.value.trim() !== "") {
					socket.send(input.value);
					input.value = "";
				}
			}
		</script>
	</body>
	</html>
	`

	t := template.Must(template.New("index").Parse(tmpl))

	mu.Lock()
	defer mu.Unlock()
	t.Execute(w, nil)
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket error:", err)
		return
	}
	defer ws.Close()

	clients[ws] = true
	log.Println("Client connected")
	// Send previous messages
	mu.Lock()
	for _, msg := range messages {
		ws.WriteMessage(websocket.TextMessage, []byte(msg))
	}
	mu.Unlock()

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			delete(clients, ws)
			break
		}
		mu.Lock()
		messages = append(messages, string(msg))
		mu.Unlock()
		broadcast <- string(msg)
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		log.Println("Message received:", msg)
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, []byte(msg))
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	}
}
