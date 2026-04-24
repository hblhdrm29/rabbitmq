package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"github.com/go-ldap/ldap/v3"
)

type User struct {
	ID       string `json:"id"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Email    string `json:"email"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

var db *sql.DB
var jwtSecret []byte

func authenticateLDAP(username, password string) (string, bool) {
	ldapURL := os.Getenv("LDAP_URL")
	if ldapURL == "" || ldapURL == "mock" {
		log.Printf("[LDAP MOCK] Authenticating user: %s\n", username)
		// Mock: Allow 'admin' or any user with password 'password123'
		if (username == "admin" && password == "admin123") || password == "password123" {
			log.Printf("[LDAP MOCK] Success for user: %s\n", username)
			return "ldap-user-" + username, true
		}
		return "", false
	}

	l, err := ldap.DialURL(ldapURL)
	if err != nil {
		log.Printf("[LDAP] Connection error: %v\n", err)
		return "", false
	}
	defer l.Close()

	// Bind with a read-only user (if required) or direct bind
	userDN := fmt.Sprintf("cn=%s,%s", username, os.Getenv("LDAP_BASE_DN"))
	err = l.Bind(userDN, password)
	if err != nil {
		log.Printf("[LDAP] Bind failed for %s: %v\n", username, err)
		return "", false
	}

	return "ldap-" + username, true
}

func main() {
	godotenv.Load(".env")
	godotenv.Load("../.env")
	godotenv.Load("../../.env")
	godotenv.Load("../../../.env")

	jwtSecret = []byte(os.Getenv("JWT_SECRET"))
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

	// Register Endpoint
	r.POST("/register", func(c *gin.Context) {
		var user User
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}

		user.ID = uuid.New().String()
		_, err = db.Exec("INSERT INTO users (id, username, password, email) VALUES (?, ?, ?, ?)",
			user.ID, user.Username, string(hashedPassword), user.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user (username might be taken)"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully", "id": user.ID})
	})

	// Login Endpoint
	r.POST("/login", func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var id, hashedPassword string
		
		// 1. Try LDAP Authentication first
		ldapID, success := authenticateLDAP(req.Username, req.Password)
		if success {
			log.Printf("Login success via LDAP: %s\n", req.Username)
			id = ldapID
		} else {
			// 2. Fallback to Database
			err := db.QueryRow("SELECT id, password FROM users WHERE username = ?", req.Username).Scan(&id, &hashedPassword)
			if err != nil {
				log.Printf("Login failed: User %s not found in LDAP or DB\n", req.Username)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
				return
			}

			err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password))
			if err != nil {
				log.Printf("Login failed: Incorrect password for user %s\n", req.Username)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
				return
			}
			log.Printf("Login success via Database: %s\n", req.Username)
		}

		// Generate JWT Token
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id":  id,
			"username": req.Username,
			"exp":      time.Now().Add(time.Hour * 24).Unix(),
		})

		tokenString, err := token.SignedString(jwtSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": tokenString, "user_id": id})
	})

	port := os.Getenv("AUTH_PORT")
	if port == "" {
		port = "8081"
	}
	fmt.Printf("Auth Service running on :%s\n", port)
	r.Run(":" + port)
}
