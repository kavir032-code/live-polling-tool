package main

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type App struct {
	DB        *mongo.Database
	Redis     *redis.Client
	Hub       *Hub
	JWTSecret []byte
}

func main() {
	_ = godotenv.Load()

	port := getEnv("PORT", "8080")
	mongoURI := getEnv("MONGO_URI", "mongodb+srv://kavi123:kaviarasan@kavi.p55gtuz.mongodb.net/?appName=Kavi")
	dbName := getEnv("MONGO_DB", "polling_app")
	jwtSecret := getEnv("JWT_SECRET", "change-this-secret")
	frontendURL := getEnv("FRONTEND_URL", "http://localhost:5173")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal("mongo connect:", err)
	}
	if err := mongoClient.Ping(ctx, nil); err != nil {
		log.Fatal("mongo ping:", err)
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Println("WARNING: REDIS_URL environment variable is empty. Using hardcoded Upstash connection string.")
		redisURL = "rediss://default:gQAAAAAABFecAAIgcDFkNzk1YTUyODNhNTI0Zjk3ODg5ZjAxNWNlZWEzYmRmNA@pleasing-martin-284572.upstash.io:6379"
	}

	redisOptions, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("invalid REDIS_URL format: %v", err)
	}

	host, _, splitErr := net.SplitHostPort(redisOptions.Addr)
	if splitErr != nil {
		host = redisOptions.Addr
	}

	redisOptions.TLSConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: host,
	}

	log.Printf("Connecting to Redis Host: %s | TLS ServerName: %s", redisOptions.Addr, redisOptions.TLSConfig.ServerName)

	rdb := redis.NewClient(redisOptions)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}
	log.Println("Redis connected successfully!")

	hub := NewHub(rdb)
	app := &App{
		DB:        mongoClient.Database(dbName),
		Redis:     rdb,
		Hub:       hub,
		JWTSecret: []byte(jwtSecret),
	}

	go hub.Run()

	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{frontendURL},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	router.POST("/api/auth/signup", app.Signup)
	router.POST("/api/auth/login", app.Login)

	router.POST("/api/polls", app.AuthMiddleware(), app.CreatePoll)
	router.GET("/api/polls/:id", app.GetPoll)
	router.GET("/api/polls/:id/results", app.GetResults)
	router.POST("/api/polls/:id/vote", app.Vote)

	router.GET("/api/polls/:id/ws", app.WebSocket)

	log.Printf("backend running on http://localhost:%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
