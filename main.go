package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Bank int

const (
	UNKNOWN     Bank = 0
	BOFA_CHECK  Bank = 1
	BOFA_CREDIT Bank = 2
	DISCOVER    Bank = 3
	CHASE       Bank = 4
)

func (b Bank) String() string {
	return [...]string{"Unknown", "BOFA_CHECK", "BOFA_CREDIT", "DISCOVER", "CHASE"}[b]
}

const (
	layoutCSVLong  = "01/02/2006"
	layoutCSVShort = "1/2/06"
)

func getRecordFmt(bank_type Bank) (map[string]int, error) {
	record_fmt := map[string]int{}
	switch bank_type {
	case BOFA_CHECK:
		record_fmt["tdate"] = 0
		record_fmt["description"] = 1
		record_fmt["amount"] = 2
	case BOFA_CREDIT:
		record_fmt["tdate"] = 0
		record_fmt["description"] = 2
		record_fmt["amount"] = 4
	case DISCOVER:
		record_fmt["tdate"] = 0
		record_fmt["description"] = 2
		record_fmt["amount"] = 3
	case CHASE:
		record_fmt["tdate"] = 0
		record_fmt["description"] = 2
		record_fmt["amount"] = 5
	default:
		return nil, fmt.Errorf("Failed to get record format of bank %d\n", bank_type)
	}
	return record_fmt, nil
}

func getBankStmtRecords(fileAbsPath string) ([][]string, error) {

	stmtFileReader, err := os.Open(fileAbsPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to open file %s", fileAbsPath)
	}
	defer stmtFileReader.Close()

	rows, err := csv.NewReader(stmtFileReader).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("Failed to process data from file %s\n", fileAbsPath)
	}

	return rows, err
}

func getBankType(filename string) (Bank, error) {
	is_checking := strings.Contains(filename, "check")
	bank_type := UNKNOWN
	if strings.HasPrefix(filename, "discover") {
		bank_type = DISCOVER
	} else if strings.Contains(filename, "chase") {
		bank_type = CHASE
	} else if strings.Contains(filename, "bofa") {
		if is_checking {
			bank_type = BOFA_CHECK
		} else {
			bank_type = BOFA_CREDIT
		}
	}

	if bank_type == UNKNOWN {
		return bank_type, fmt.Errorf("Failed to get bank type for filename %s\n", filename)
	}

	return bank_type, nil
}

// Add filters here
func skip_record_rules(record []string, record_fmt map[string]int, month int, year int) (bool, string) {
	var skipReason string
	skip := false

	for {
		if tDate, err := time.Parse(layoutCSVLong, record[record_fmt["tDate"]]); err != nil {
			if tDate1, err1 := time.Parse(layoutCSVShort, record[record_fmt["tDate"]]); err1 != nil {
				skip = true
				skipReason = "transaction date error"
				break
			} else {
				if int(tDate1.Month()) != month {
					log.Printf("INFO: Month Mismatch1: tDate: IN: %s, OUT: %s\n",
						record[record_fmt["tDate"]], tDate.Format(layoutCSVShort))
					skip = true
					skipReason = "transaction date month mismatch"
					break
				}
			}
		} else {
			if int(tDate.Month()) != month {
				skip = true
				skipReason = "transaction date month mismatch"
				break
			}
		}

		// if strings.Contains(record[record_fmt["description"]], "ARUBA") == true {
		// 	skip = true
		// 	skipReason = "salary deposit"
		// 	break
		// }

		break
	}

	return skip, skipReason
}

func UpdateExpenseReport(fileAbsPath string, records []string) (int, error) {
	var num int
	var err error

	dirName := filepath.Dir(fileAbsPath)
	fileName := filepath.Base(fileAbsPath)
	outFileName := dirName + "/output/output.csv"
	log.Printf("INFO: Writing records to output file: %s\n", outFileName)

	outFile, err := os.OpenFile(outFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer outFile.Close()

	if _, err := outFile.WriteString(fmt.Sprintf("\n%s\n\n", fileName)); err != nil {
		log.Fatalf("Failed to write to output file: %s", err)
	}

	for _, record := range records {
		log.Printf("INFO: writing record: %s\n", record)
		if _, err := outFile.Write([]byte(record)); err != nil {
			log.Fatalf("Failed to write record: %v : err: %s\n", record, err)
		}
		num++
	}

	return num, err
}

func ProcessBankStmt(fileAbsPath string, month int, year int) {

	bank_type, err := getBankType(filepath.Base(fileAbsPath))
	if err != nil {
		log.Fatalln(err)
	}

	records, err := getBankStmtRecords(fileAbsPath)
	if err != nil {
		log.Fatalln(err)
	}

	if bank_type == BOFA_CHECK {
		log.Println("Skip 7 rows of bofa checking bank stmt\n")
		records = records[7:]
	}

	record_fmt, err := getRecordFmt(bank_type)
	if err != nil {
		log.Fatalf("Failed to get record format for bank_type %s\n", bank_type.String())
	}

	output_records := []string{}
	var subtotal float64
	for index, record := range records {
		// Skip header
		if index == 0 {
			log.Println("Skip header")
			continue
		}

		if skip, reason := skip_record_rules(record, record_fmt, month, year); skip {
			log.Printf("INFO: Skip record: %v --> %s\n", record, reason)
			continue
		}

		output_records = append(output_records, fmt.Sprintf("%s,%s,%s\n", record[record_fmt["tDate"]],
			record[record_fmt["description"]], record[record_fmt["amount"]]))
	}

	output_records = append(output_records, fmt.Sprintf("\nSubtotal,,\n\n"))
	num, err := UpdateExpenseReport(fileAbsPath, output_records)
	if err != nil {
		log.Fatalf("Failed to write %d records to expense report\n", num)
	}

	log.Printf("Successfuly wrote %d records to expense report\n", num)
}

func ReadExpenseReports(pathdir string, month int, year int) {
	filesInfo, err := ioutil.ReadDir(pathdir)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range filesInfo {
		if !file.IsDir() {
			if extn := filepath.Ext(file.Name()); extn == ".csv" || extn == ".CSV" {
				log.Printf("INFO: Processing statement %s\n", file.Name())
				ProcessBankStmt(pathdir+"/"+file.Name(), month, year)
			}
		}
	}
}

func main() {
	// Read CSV bank statements
	cyear, cmonth, _ := time.Now().Date()
	pathDirD := flag.String("path", "location of bank statements in CSV", "/path/to/bank_stmt/*.csv")
	month := flag.Int("month", int(cmonth), "1-12")
	year := flag.Int("year", cyear, "YYYY")

	flag.Parse()

	log.Printf("INFO: Generating expense report for %d/%d\n", *month, *year)

	ReadExpenseReports(*pathDirD, int(*month), *year)
}
