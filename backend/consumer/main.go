package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Transaction struct {
	ID           string  `json:"id"`
	Amount       float64 `json:"amount"`
	MerchantName string  `json:"merchant_name"`
}

func main() {
	// Muat .env secara eksplisit
	godotenv.Load(".env")
	godotenv.Load("../.env")
	godotenv.Load("../../.env")

	// Connect to MySQL
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPass, dbHost, dbPort, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Connect to RabbitMQ
	rmqUser := os.Getenv("RABBITMQ_USER")
	rmqPass := os.Getenv("RABBITMQ_PASS")
	rmqHost := os.Getenv("RABBITMQ_HOST")
	rmqPort := os.Getenv("RABBITMQ_PORT")

	rmqURL := fmt.Sprintf("amqp://%s:%s@%s:%s/", rmqUser, rmqPass, rmqHost, rmqPort)
	var conn *amqp.Connection
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

	// Create persistent queue for Consumer
	q, err := ch.QueueDeclare("consumer_queue", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Bind queue to exchange
	err = ch.QueueBind(q.Name, "", "transaction_exchange", false, nil)
	if err != nil {
		log.Fatal(err)
	}

	msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Payment Consumer running... Waiting for transactions.")

	for d := range msgs {
		log.Printf("Received message: Type=%s, Body=%s\n", d.Type, string(d.Body))

		if d.Type != "TRANSACTION_CREATED" {
			log.Println("Skipping message: not a TRANSACTION_CREATED event")
			d.Ack(false)
			continue
		}

		var t Transaction
		json.Unmarshal(d.Body, &t)

		fmt.Printf("Processing payment for: %s (ID: %s)...\n", t.MerchantName, t.ID)

		// Simulasi proses perbankan (2 detik)
		time.Sleep(2 * time.Second)

		// Update status di database jadi SUCCESS
		_, err = db.Exec("UPDATE transactions SET status = 'SUCCESS' WHERE id = ?", t.ID)
		if err != nil {
			log.Println("Failed to update status:", err)
			d.Nack(false, true) // Retry if failed
			continue
		}

		fmt.Printf("Transaction %s successfully processed!\n", t.ID)

		// Kirim event status update ke RabbitMQ agar WebSocket bisa memberitahu Frontend
		statusEvent := map[string]interface{}{
			"id":     t.ID,
			"status": "SUCCESS",
		}
		payload, _ := json.Marshal(statusEvent)
		
		err = ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType: "application/json",
			Body:        payload,
			Type:        "TRANSACTION_STATUS_UPDATED",
		})

		if err != nil {
			log.Println("Failed to publish status update:", err)
		}

		d.Ack(false)
	}
}
