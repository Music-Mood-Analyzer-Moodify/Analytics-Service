package repository

import (
	"analytics-service/internal/util"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq" // PostgreSQL driver
)

var (
  host     = os.Getenv("POSTGRES_HOST")
  port     = os.Getenv("POSTGRES_PORT")
  user     = os.Getenv("POSTGRES_USER")
  password = os.Getenv("POSTGRES_PASSWORD")
  dbname   = os.Getenv("POSTGRES_DB")
)

func InitTables() error {
	db, err := initDB()
	if err != nil {
		return err
	}
	defer db.Close()

	err = createTables(db)
	if err != nil {
		return err
	}

	return nil
}

func CreateMessage(message string) (int, error) {
	db, err := initDB()
	util.FailOnError(err, "Failed to initialize database")
	defer db.Close()

	stmt, err := db.Prepare("INSERT INTO messages (message) VALUES ($1) RETURNING id")
	util.FailOnError(err, "Failed to prepare statement")
	defer stmt.Close()

	var id int
	err = stmt.QueryRow(message).Scan(&id)
	util.FailOnError(err, "Failed to execute statement")

	return id, nil
}

type message struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
}

func GetMessages() ([]message, error) {
	db, err := initDB()
	util.FailOnError(err, "Failed to initialize database")
	defer db.Close()
	
	rows, err := db.Query("SELECT * FROM messages")
	util.FailOnError(err, "Failed to query messages")
	defer rows.Close()

	var messages []message
	for rows.Next() {
		var message = message{}
		err := rows.Scan(&message.ID, &message.Message)
		util.FailOnError(err, "Failed to scan message")
		messages = append(messages, message)
	}

	util.FailOnError(rows.Err(), "Failed to iterate over messages")
	return messages, nil
}

func initDB() (*sql.DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	db, err := sql.Open("postgres", psqlInfo)
	util.FailOnError(err, "Failed to open database connection")

	if db == nil {
		util.FailOnError(err, "Database connection is nil")
	}

	err = db.Ping()
	util.FailOnError(err, "Failed to ping database")

	return db, nil
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id SERIAL PRIMARY KEY,
			message	TEXT NOT NULL
		);
	`)
	util.FailOnError(err, "Failed to create messages table")

	return nil
}