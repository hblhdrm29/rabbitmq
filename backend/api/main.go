package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type Transaction struct {
	ID           string  `json:"id"`
	Amount       float64 `json:"amount"`
	Description  string  `json:"description"`
	Status       string  `json:"status"`
	MerchantName string  `json:"merchant_name"`
	UserID       string  `json:"user_id"`
	CreatedAt    string  `json:"created_at"`
	PaymentMethod string `json:"payment_method"`
}

var db *sql.DB

func main() {
	// Mencari file .env di folder ini atau folder di atasnya
	godotenv.Load(".env")
	godotenv.Load("../.env")
	godotenv.Load("../../.env")

	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("default-secret-key")
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", dbUser, dbPass, dbHost, dbPort, dbName)
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	r := gin.Default()
	
	// Custom CORS configuration to allow Authorization header
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Middleware JWT
	authMiddleware := func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		tokenString := ""
		fmt.Sscanf(authHeader, "Bearer %s", &tokenString)
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format"})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		c.Set("user_id", claims["user_id"])
		c.Next()
	}

	// Grouping routes that require authentication
	authorized := r.Group("/")
	authorized.Use(authMiddleware)

	// Endpoint untuk mengambil data transaksi (Pagination)
	authorized.GET("/transactions", func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
		offset := (page - 1) * limit

		rows, err := db.Query("SELECT id, amount, description, status, merchant_name, user_id, created_at, COALESCE(payment_method, 'CASH') FROM transactions ORDER BY created_at DESC LIMIT ? OFFSET ?", limit, offset)
		if err != nil {
			log.Println("Query Error:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var transactions []Transaction
		for rows.Next() {
			var t Transaction
			rows.Scan(&t.ID, &t.Amount, &t.Description, &t.Status, &t.MerchantName, &t.UserID, &t.CreatedAt, &t.PaymentMethod)
			transactions = append(transactions, t)
		}

		// Get total count for pagination metadata
		var total int
		db.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&total)

		c.JSON(http.StatusOK, gin.H{
			"data":  transactions,
			"total": total,
			"page":  page,
			"limit": limit,
		})
	})

	// Endpoint untuk membuat transaksi baru (Transactional Outbox Pattern)
	authorized.POST("/transactions", func(c *gin.Context) {
		var t Transaction
		if err := c.ShouldBindJSON(&t); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		t.ID = uuid.New().String()
		t.Status = "PENDING"
		// Use user_id from token if not provided
		if t.UserID == "" {
			if uid, exists := c.Get("user_id"); exists {
				t.UserID = uid.(string)
			} else {
				t.UserID = uuid.New().String()
			}
		}

		// Mulai transaksi database
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 1. Simpan ke tabel transactions
		_, err = tx.Exec("INSERT INTO transactions (id, amount, description, status, merchant_name, user_id, payment_method) VALUES (?, ?, ?, ?, ?, ?, ?)",
			t.ID, t.Amount, t.Description, "PENDING", t.MerchantName, t.UserID, t.PaymentMethod)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 2. Simpan ke tabel outbox
		payload, _ := json.Marshal(t)
		_, err = tx.Exec("INSERT INTO outbox (event_type, payload) VALUES (?, ?)", "TRANSACTION_CREATED", payload)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Commit transaksi
		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, t)
	})

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("API Service running on :%s\n", port)
	r.Run(":" + port)
}
