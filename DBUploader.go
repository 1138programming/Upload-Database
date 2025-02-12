package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/go-sql-driver/mysql"
)

var db *sql.DB

func main() {

	cfg := mysql.Config{
		User: "root",
		Passwd: "1138",
		Net: "tcp",
		Addr: "127.0.0.1:3306",
		DBName: "1138scapp",
		AllowNativePasswords: true,
	}

	//database handle
	var err error
	
	db, err = sql.Open("mysql", cfg.FormatDSN())

	if err != nil {
		log.Fatal(err)
	}

	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}

	fmt.Println("Database connected")
}

//googlesheets -> credit card down but no charge
//Use free cloud DB with limited rows
//Server in tema room accessed using VPN
