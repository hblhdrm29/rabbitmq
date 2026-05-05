package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/segmentio/kafka-go"
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
	err = ch.ExchangeDeclare("transaction_exchange", "fanout", true, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Connect to Kafka
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "localhost:9092"
	}
	kafkaWriter := &kafka.Writer{
		Addr:     kafka.TCP(kafkaBroker),
		Topic:    "transaction-events",
		Balancer: &kafka.LeastBytes{},
	}
	defer kafkaWriter.Close()

	fmt.Println("Outbox Worker running (RabbitMQ + Kafka)...")

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

			// 1. Publish ke RabbitMQ (Fanout)
			err = ch.Publish("transaction_exchange", "", false, false, amqp.Publishing{
				ContentType: "application/json",
				Body:        payload,
				Type:        eventType,
			})
			if err != nil {
				log.Println("Failed to publish to RabbitMQ:", err)
			}

			// 2. Publish ke Kafka
			err = kafkaWriter.WriteMessages(context.Background(), kafka.Message{
				Key:   []byte(fmt.Sprintf("%d", id)),
				Value: payload,
			})
			if err != nil {
				log.Println("Failed to publish to Kafka:", err)
			}

			// Update status di outbox
			_, err = db.Exec("UPDATE outbox SET status = 'PROCESSED' WHERE id = ?", id)
			if err != nil {
				log.Println("Failed to update outbox status:", err)
			} else {
				fmt.Printf("Published event %d to RabbitMQ & Kafka\n", id)
			}
		}
		rows.Close()

		time.Sleep(1 * time.Second)
	}
}
