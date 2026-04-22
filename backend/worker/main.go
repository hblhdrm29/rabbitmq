package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

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
	var err error
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

	// Declare Fanout Exchange
	err = ch.ExchangeDeclare(
		"transaction_exchange", // name
		"fanout",               // type
		true,                   // durable
		false,                  // auto-deleted
		false,                  // internal
		false,                  // no-wait
		nil,                    // arguments
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Outbox Worker running (Fanout Mode)...")

	for {
		// Polling tabel outbox
		rows, err := db.Query("SELECT id, event_type, payload FROM outbox WHERE status = 'PENDING' LIMIT 10")
		if err != nil {
			log.Println("Error querying outbox:", err)
			time.Sleep(2 * time.Second)
			continue
		}

		for rows.Next() {
			var id int
			var eventType string
			var payload []byte
			rows.Scan(&id, &eventType, &payload)

			// Publish ke Exchange (Fanout)
			err = ch.Publish(
				"transaction_exchange", // exchange
				"",                     // routing key (ignored in fanout)
				false,                  // mandatory
				false,                  // immediate
				amqp.Publishing{
					ContentType: "application/json",
					Body:        payload,
					Type:        eventType,
				})

			if err != nil {
				log.Println("Failed to publish message:", err)
				continue
			}

			// Update status di outbox
			_, err = db.Exec("UPDATE outbox SET status = 'PROCESSED' WHERE id = ?", id)
			if err != nil {
				log.Println("Failed to update outbox status:", err)
			} else {
				fmt.Printf("Published event %d to RabbitMQ\n", id)
			}
		}
		rows.Close()

		time.Sleep(1 * time.Second)
	}
}
