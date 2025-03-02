//Compile with command GOOS=windows GOARCH=amd64 go build -o {versionName}.exe

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

var db *sql.DB

type Database struct {
	Name string
	User string
	Password string
	Network string
	Address string
	AllowNativePasswords bool
	TableNames []string
}

type GoogleSheet struct {
	SpreadsheetId string
	Sheets []string 
}

func main() {


	//------------ 1138scapp DB setup ----------//

	var DB_1138scapp Database
	//1138scapp values
	DB_1138scapp.Name = "1138scapp"
	DB_1138scapp.User = "root"
	DB_1138scapp.Password = "1138"
	DB_1138scapp.Network = "tcp"
	DB_1138scapp.Address = "127.0.0.1:3306"
	DB_1138scapp.AllowNativePasswords = true

	//Table names
	DB_1138scapp.TableNames = []string{
		"datapoint",
		"alliance",
		"matchtransaction",
		"scmatch",
		"scouter",
		"team",
		"user",
	}

	db := connectToDb(
		DB_1138scapp.User, 
		DB_1138scapp.Password, 
		DB_1138scapp.Network, 
		DB_1138scapp.Address, 
		DB_1138scapp.Name, 
		DB_1138scapp.AllowNativePasswords,
	)
	//-----------------------------------//

	//-------- GoogleSheet setup --------//

	var GoogleSheet GoogleSheet

	GoogleSheet.SpreadsheetId = "1kMpYNbYvbXHQywKSepXi9q8w7wwz8GP90rn9bIxd8Ck"
	GoogleSheet.Sheets = []string{
		"datapoint",
		"alliance",
		"matchtransaction",
		"scmatch",
		"scouter",
		"team",
	}

	//-----------------------------------//

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

	//----- Getting the last IDs of data present in the sheet ----//

	//configure ranges
	var ranges []string

	for _, name := range GoogleSheet.Sheets {
		ranges = append(ranges, name + "!2:9999")
	}

	//configure last Ids of each sheet
	lastIds := getLastKeyFromSheets(
		batchRead(srv, GoogleSheet.SpreadsheetId, ranges),
		GoogleSheet.Sheets,
	)

	var GoogleSheetRequest []*sheets.ValueRange

	//scans all tables from Database and adds them to a list of ValueRange pointers
	//This is done to batch write and only use 1 API call fro everything
	for _, name := range DB_1138scapp.TableNames {

		var err error
		IdOffset, valueExists := lastIds[name]

		if !slices.Contains(GoogleSheet.Sheets, name) {
			continue
		}
	
		
		//checks if the offset retrieved is an integer
		//if so it will convert it to a string to be used as the offset
		//else it will simply set the offset to be 0
		//the 0 case will make it so that the sheet returns to normal after everything 
		var table [][]interface{}

		if valueExists {
			fmt.Println("Getting data from database: ", name, " with offset: ", IdOffset)
			table, err = getDataFromDBTable(db, name, convertInterfaceOfIdToIntString(IdOffset))
		} else {
			fmt.Println("Error finding ID value. Putting in '0' as default")
			table, err = getDataFromDBTable(db, name, 0)
		}

		if err != nil {
			fmt.Println("Issue with retrieving table: ", name, "%q: %v", err)
			continue
		}

		writeRange := name + "!2:9999"

		GoogleSheetRequest = append(GoogleSheetRequest, &sheets.ValueRange{
			Range: writeRange,
			Values: table,
		})
	}

	batchWrite(srv, GoogleSheet.SpreadsheetId, GoogleSheetRequest)
	



	//use Values.Append for adding new data and Values.Update to replace data

}

//----------------------------------------//
//------ Sheet API & Helper Methods ------//
//----------------------------------------//

//Range is in A1 notation

func readSheet(srv *sheets.Service, spreadsheetId string, readrange string) *sheets.ValueRange {


	resp, err := srv.Spreadsheets.Values.Get(spreadsheetId, readrange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	return resp
}

//reads an entire googleSheet and returns a map of each sheet name to their table
//this is done to better organize sheets fro reading
func batchRead(srv *sheets.Service, spreadsheetId string, ranges []string) map[string][][]interface{} {
	values, err := srv.Spreadsheets.Values.BatchGet(spreadsheetId).Ranges(ranges...).ValueRenderOption("UNFORMATTED_VALUE").Do()

	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	//make a map that values can be add to without worrying about nil errors
	sheetsMap := make(map[string][][]interface{})

	//for each sheet of our google sheet, take the name and the table as key value pair
	//this makes working with this data easier
	for _, sheets := range values.ValueRanges {
		sheetsMap[strings.Split(sheets.Range, "!")[0]] = sheets.Values
	}

	return sheetsMap
}

//use Values.Append for adding new data and Values.Update to replace data
//Writes to single sheets only. Please use batchWrite to write to multiple at once if need so
func WriteSheetAdd(srv *sheets.Service, spreadsheetId string, writeRange string, values [][]interface{}) {
	
	var valueRange *sheets.ValueRange = &sheets.ValueRange{Values: values, MajorDimension: "ROWS"}
	_, err := srv.Spreadsheets.Values.Append(spreadsheetId, writeRange, valueRange).ValueInputOption("RAW").Do()

	if err != nil {
		fmt.Println("Error writing to sheet:", err)
	}
}

func batchWrite(srv *sheets.Service, spreadsheetId string, requests []*sheets.ValueRange) {

	batchUpdateRequest := &sheets.BatchUpdateValuesRequest {
		ValueInputOption: "RAW",
		Data: requests,
	}

	_, err := srv.Spreadsheets.Values.BatchUpdate(spreadsheetId, batchUpdateRequest).Do()

	if err != nil {
		log.Fatalf("unable to send batch update: %v", err)
	}
}

//Gets the value from hopefully the last row 1s column giving us the latest id
//this is used so that we can search the database for data not on the spreadsheet
//retireves the CELL VALUE not the index of the last cell value !!!
func getLastKeyFromSheets(sheetsMap map[string][][]interface{}, wantedSheetNames []string) map[string]interface{} {

	lastIds := make(map[string]interface{})

	//accesses the 1st value from the last row available 
	for _, name := range wantedSheetNames {

		//checks 
		if len(sheetsMap[name]) <= 0 {
			lastIds[name] = 0
			continue
		}

		fmt.Println("Scanning: ", name, " for data")
		lastIds[name] = sheetsMap[name][len(sheetsMap[name]) - 1][0]
		fmt.Println("Success: latest ID: ", lastIds[name])
	}

	fmt.Println("Id map: ", lastIds, "\n")
	return lastIds
}

//----------------------------------------//
//------- Authentification Methods -------//
//----------------------------------------//

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

//----------------------------------------//
//----------- Database Methods -----------//
//----------------------------------------//

func connectToDb(user string, password string, network string, address string, name string, nativePassowrds bool) *sql.DB {
	cfg := mysql.Config{
		User: user,
		Passwd: password,
		Net: network,
		Addr: address,
		DBName: name,
		AllowNativePasswords: nativePassowrds,
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

func getDataFromDBTable(db *sql.DB, tableName string, IdOffset int) ([][]interface{}, error) {
	
	//selects all rows starting from a certain id and all the ones after that 
	query := fmt.Sprintf("Select * From %s LIMIT 10000000 OFFSET %d", tableName, IdOffset)
	
	rows, err := db.Query(query)

	if err != nil {
		return nil, fmt.Errorf("error retrieiving rows %q: %v", tableName, err)
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

//used as a helper function for id to check if id
//is an string of an integer
//returns 0 as a default to reload db on sheet
func convertInterfaceOfIdToIntString(intf interface{}) int {

	s := fmt.Sprintf("%v", intf)

	i, err := strconv.Atoi(s)

	if err != nil {
		return i
	} else {
		fmt.Errorf("conversion error %q:", err)
		return 0
	}
}



//googlesheets -> credit card down but no charge
//Use free cloud DB with limited rows
//Server in tema room accessed using VPN

//Next steps: 
//Configure all tables to go to google sheet Check
//Configure only new data to go to google sheet w/loop
//	Done by reading A2 of all sheets
//Implement