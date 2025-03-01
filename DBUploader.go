//Compile with command GOOS=windows GOARCH=amd64 go build -o {versionName}.exe

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

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

	//temporary For DEBUGGING purposes only - not final product -
	var tableName string
	fmt.Scan(&tableName)
	

	table, err := getDataFromDBTable(db, tableName)
	
	if err != nil {
		fmt.Println("Issue with retrieving table: ", tableName, "%q: %v", err)
	}

	WriteSheetAdd(spreadsheetId, writeRange, table, srv)


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
	
	

	rows, err := db.Query("Select * From " + tableName)
	if err != nil {
		return nil, fmt.Errorf("table %q: %v ", tableName, err)
	}

	var table [][]interface{}

	defer rows.Close()

	//looping through rows and adding them to table interface
	//creates a temporary 1 dimensional interface of column length 
	//and appends to 2 dimensional matrix after scanning
	for rows.Next() {
		
		columns, _ := rows.Columns()
		//this interface is a row
		row := make([]interface{},len(columns))
		rowPointers := make([]interface{}, len(columns))

		for i := range columns {
			rowPointers[i] = &row[i] //allocate pointers to store values
		}

		err := rows.Scan(rowPointers...);

		if err != nil {
			return nil, fmt.Errorf("table %q: %v", tableName, err)
		}

		//Loop derefrences pointers in values and applies them to row
		//The switch statement ensures that all types are formatted correctly
		for i, col := range rowPointers {
			value := *(col.(*interface{}))

			if value == nil {
				row[i] = ""
			} else {
				switch value := value.(type) {
				case []byte: 
					strValue := string(value)
					//checks if byte array as a string represents a number -> will convert if so
					if intValue, err := strconv.ParseInt(strValue, 10, 64); err == nil {
						row[i] = intValue
						fmt.Println("Byte Array found to be Integer: ", row[i])
					} else if floatValue, err := strconv.ParseFloat(strValue, 64); err == nil {
						row[i] = floatValue
						fmt.Println("Byte Array found to be Float: ", row[i])
					} else {
						row[i] = strValue
						fmt.Println("Byte Array found to be String: ", row[i])
					}

					row[i] = string(value)
					
				case int64:
					row[i] = strconv.FormatInt(value, 10)
					fmt.Println("Integers found: ", row[i])
				case float64:
					row[i] = strconv.FormatFloat(value, 'f', -1, 64)
					fmt.Println("Floats found: ", row[i])
				case bool:
					row[i] = strconv.FormatBool(value)
					fmt.Println("Booleans found: ", row[i])
				default:
					row[i] = fmt.Sprintf("%v", value)
					fmt.Println("Something else found: ", row[i])
				}
			}

			
		}
	

		table = append(table, row)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf(tableName, "%q: %v", err)
	}
	
	fmt.Println("Database data succesfully retrieved from: " + tableName)
	fmt.Println(table)
	return table, nil
}

func String2dToInterface2d(arr [][]string) [][]interface{} {

	intfc := make([][]interface{}, len(arr)) 

	for i, row := range arr {
		intfc[i] = make([]interface{}, len(row))
		for j, val := range row {
			intfc[i][j] = val
		}
	}

	return intfc
}


//googlesheets -> credit card down but no charge
//Use free cloud DB with limited rows
//Server in tema room accessed using VPN