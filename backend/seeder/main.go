package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

const (
	totalRecords = 1000000
	batchSize    = 10000
)

func main() {
	// Mencari file .env di folder ini atau folder di atasnya
	godotenv.Load(".env")
	godotenv.Load("../.env")
	godotenv.Load("../../.env")

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

	// Wait for DB to be ready
	for i := 0; i < 10; i++ {
		err = db.Ping()
		if err == nil {
			break
		}
		fmt.Println("Waiting for database...")
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatal("Could not connect to DB:", err)
	}

	fmt.Println("Starting to seed 1,000,000 records...")
	start := time.Now()

	merchants := []string{"Indomaret", "Alfamart", "Shopee", "Tokopedia", "Grab", "Gojek"}
	statuses := []string{"SUCCESS", "PENDING", "FAILED"}

	for i := 0; i < totalRecords; i += batchSize {
		valueStrings := make([]string, 0, batchSize)
		valueArgs := make([]interface{}, 0, batchSize*6)

		for j := 0; j < batchSize; j++ {
			valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?)")
			valueArgs = append(valueArgs, uuid.New().String())
			valueArgs = append(valueArgs, rand.Float64()*1000000)
			valueArgs = append(valueArgs, fmt.Sprintf("Transaction %d", i+j))
			valueArgs = append(valueArgs, statuses[rand.Intn(len(statuses))])
			valueArgs = append(valueArgs, merchants[rand.Intn(len(merchants))])
			valueArgs = append(valueArgs, uuid.New().String())
		}

		stmt := fmt.Sprintf("INSERT INTO transactions (id, amount, description, status, merchant_name, user_id) VALUES %s",
			strings.Join(valueStrings, ","))

		_, err := db.Exec(stmt, valueArgs...)
		if err != nil {
			log.Fatal(err)
		}

		if (i+batchSize)%50000 == 0 {
			fmt.Printf("Inserted %d records...\n", i+batchSize)
		}
	}

	fmt.Printf("Seeding completed in %v\n", time.Since(start))
}
