//Compile with command GOOS=windows GOARCH=amd64 go build -o {versionName}.exe

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var db *sql.DB

func main() {

	db := connectToDb()

	//---Connect to sheet---//
	ctx := context.Background()
	b, err := os.ReadFile("API_credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.JWTConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets") 
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	client := config.Client(oauth2.NoContext)

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}
	//----------------------//

	//Important edit info here
	spreadsheetId := "1kMpYNbYvbXHQywKSepXi9q8w7wwz8GP90rn9bIxd8Ck"
	writeRange := "Sheet1!A1:A1"
	tableName := "team"
	

	values, err := getDataFromDBTable(db, tableName)
	if err != nil {
		fmt.Println("Issue with retrieving table: ", tableName, "%q: %v", err)
	}

	WriteSheetAdd(spreadsheetId, writeRange, values, srv)


	//use Values.Append for adding new data and Values.Update to replace data

}

//Range is in A1 notation
func readSheet(spreadsheetId string, readrange string, srv *sheets.Service) *sheets.ValueRange {


	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readrange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	return resp
}

//use Values.Append for adding new data and Values.Update to replace data
func WriteSheetAdd(spreadsheetId string, writeRange string, values [][]interface{}, srv *sheets.Service) {
	
	var valueRange *sheets.ValueRange = &sheets.ValueRange{Values: values, MajorDimension: "ROWS"}
	_, err := srv.Spreadsheets.Values.Append(spreadsheetId, writeRange, valueRange).ValueInputOption("RAW").Do()

	if err != nil {
		fmt.Println("Error writing to sheet:", err)
	}
}

func getClient(config *oauth2.Config) *http.Client {

	tokFile := "token.json"
	tok := tokenFromFile(tokFile)
	return config.Client(context.Background(), tok)

}

func tokenFromFile(file string) (*oauth2.Token) {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()
	tok := &oauth2.Token{}
	//err = json.NewDecoder(f).Decode(tok)
	return tok
}

func connectToDb() *sql.DB {
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
	return db
}

func getDataFromDBTable(db *sql.DB, tableName string) ([][]interface{}, error) {
	
	var table [][]interface{}

	rows, err := db.Query("Select * From " + tableName)
	if err != nil {
		return nil, fmt.Errorf("Table: ", tableName, "%q: %v", err)
	}

	defer rows.Close()

	//looping through rows and adding them to table interface
	//creates a temporary 1 dimensional interface of column length 
	//and appends to 2 dimensional matrix after scanning
	for rows.Next() {
		
		columnCount, _ := rows.Columns()
		row := make([]interface{}, len(columnCount))

		//have another interface of pointers to values in "row"
		//ensures that row can pretty much be passed without knowing columns
		//and being able to be passed as 1 arguement
		rowPointers := make([]interface{}, len(columnCount))

		for i := range row {
			rowPointers[i] = &row[i]
		}

		err := rows.Scan(rowPointers...);
		table = append(table, row)
		if err != nil {
			return nil, fmt.Errorf("table %q: %v", tableName, err)
		}
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf(tableName, "%q: %v", err)
	}
	
	fmt.Println("Database data succesfully retrieved from: " + tableName)
	fmt.Println(table)
	return table, nil
}


//googlesheets -> credit card down but no charge
//Use free cloud DB with limited rows
//Server in tema room accessed using VPN