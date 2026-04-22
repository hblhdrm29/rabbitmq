package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for demo
	},
}

type Client struct {
	Conn *websocket.Conn
}

var (
	clients   = make(map[*Client]bool)
	clientsMu sync.Mutex
)

func main() {
	// Muat .env secara eksplisit
	godotenv.Load(".env")
	godotenv.Load("../.env")
	godotenv.Load("../../.env")

	// Connect to RabbitMQ
	rmqUser := os.Getenv("RABBITMQ_USER")
	rmqPass := os.Getenv("RABBITMQ_PASS")
	rmqHost := os.Getenv("RABBITMQ_HOST")
	rmqPort := os.Getenv("RABBITMQ_PORT")

	rmqURL := fmt.Sprintf("amqp://%s:%s@%s:%s/", rmqUser, rmqPass, rmqHost, rmqPort)
	var conn *amqp.Connection
	var err error
	for i := 0; i < 10; i++ {
		conn, err = amqp.Dial(rmqURL)
		if err == nil {
			break
		}
		fmt.Println("Waiting for RabbitMQ...")
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer ch.Close()

	// Declare Exchange
	err = ch.ExchangeDeclare("transaction_exchange", "fanout", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Create private queue for WS
	q, err := ch.QueueDeclare("", false, false, true, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Bind queue to exchange
	err = ch.QueueBind(q.Name, "", "transaction_exchange", false, nil)
	if err != nil {
		log.Fatal(err)
	}

	msgs, err := ch.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Listen for RabbitMQ messages in background
	go func() {
		for d := range msgs {
			fmt.Printf("Received message from RabbitMQ, broadcasting to %d clients\n", len(clients))
			broadcast(d.Body)
		}
	}()

	// WebSocket handler
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("Upgrade error:", err)
			return
		}
		
		client := &Client{Conn: ws}
		clientsMu.Lock()
		clients[client] = true
		clientsMu.Unlock()
		
		fmt.Println("New client connected")

		// Keep connection open and clean up on close
		defer func() {
			clientsMu.Lock()
			delete(clients, client)
			clientsMu.Unlock()
			ws.Close()
			fmt.Println("Client disconnected")
		}()

		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				break
			}
		}
	})

	port := os.Getenv("WS_PORT")
	if port == "" {
		port = "8086"
	}
	fmt.Printf("WS Gateway running on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func broadcast(message []byte) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for client := range clients {
		err := client.Conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Println("Write error:", err)
			client.Conn.Close()
			delete(clients, client)
		}
	}
}
