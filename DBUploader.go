package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/go-sql-driver/mysql"
)

var db *sql.DB

func main() {

	cfg := mysql.Config{
		User: os.Getenv("root"),
		Passwd: os.Getenv("1138"),
		Net: "tcp",
		Addr: "127.0.0.1:0",
		DBName: "1138scapp",
	}

	//database handle
	var err error
	
	db, err = sql.Open("mySql", cfg.FormatDSN())

	if err != nil {
		log.Fatal(err)
	}

	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}

	fmt.Println("Database connected")
}