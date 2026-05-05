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
	ID            string  `json:"id"`
	Amount        float64 `json:"amount"`
	Description   string  `json:"description"`
	MerchantName  string  `json:"merchant_name"`
	UserID        string  `json:"user_id"`
	PaymentMethod string  `json:"payment_method"`
	Status        string  `json:"status"`
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

	// Create persistent queue for Web 1 Consumer
	q, err := ch.QueueDeclare("web1_consumer_queue", true, false, false, false, nil)
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

	fmt.Printf("Web 1 Consumer running (DB: %s)... Waiting for messages.\n", dbName)

	for d := range msgs {
		log.Printf("Received message: Type=%s\n", d.Type)

		if d.Type == "TRANSACTION_CREATED" {
			var t Transaction
			json.Unmarshal(d.Body, &t)

			log.Printf("Syncing transaction to local DB: %s\n", t.ID)
			// INSERT IGNORE: jika sudah ada (dari API sendiri), skip
			_, err = db.Exec("INSERT IGNORE INTO transactions (id, amount, description, status, merchant_name, user_id, payment_method) VALUES (?, ?, ?, ?, ?, ?, ?)",
				t.ID, t.Amount, t.Description, "PENDING", t.MerchantName, t.UserID, t.PaymentMethod)
			if err != nil {
				log.Println("Failed to sync transaction:", err)
			}

			// Simulasi proses perbankan (2 detik)
			time.Sleep(2 * time.Second)

			// Update status di database jadi SUCCESS
			_, err = db.Exec("UPDATE transactions SET status = 'SUCCESS' WHERE id = ?", t.ID)
			if err != nil {
				log.Println("Failed to update status:", err)
			}

			// Kirim event status update ke RabbitMQ
			statusEvent := map[string]interface{}{
				"id":     t.ID,
				"status": "SUCCESS",
			}
			payload, _ := json.Marshal(statusEvent)
			ch.Publish("transaction_exchange", "", false, false, amqp.Publishing{
				ContentType: "application/json",
				Body:        payload,
				Type:        "TRANSACTION_STATUS_UPDATED",
			})

			d.Ack(false)
			continue
		}

		if d.Type == "PRODUCT_CREATED" {
			log.Println("Received product update")
			d.Ack(false)
			continue
		}

		if d.Type == "TRANSACTION_STATUS_UPDATED" {
			var status map[string]interface{}
			json.Unmarshal(d.Body, &status)
			if id, ok := status["id"].(string); ok {
				db.Exec("UPDATE transactions SET status = 'SUCCESS' WHERE id = ?", id)
			}
			d.Ack(false)
			continue
		}

		d.Ack(false)
	}
}
