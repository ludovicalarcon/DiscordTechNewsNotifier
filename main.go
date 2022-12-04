package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

func clearDbForNewDay(firstLine string, filePath string, file *os.File) *os.File {
	const dateLayout = "2006-01-02"

	currentDate := time.Now().UTC()
	dbDate, err := time.Parse(dateLayout, firstLine)
	isNeededToClear := false

	if err != nil {
		log.Fatalln(err)
	}

	if currentDate.Year() != dbDate.Year() || currentDate.Month() != dbDate.Month() || currentDate.Day() != dbDate.Day() {
		isNeededToClear = true
	}

	if isNeededToClear {
		data := []byte(fmt.Sprintf("%v\n", currentDate.Format(dateLayout)))
		err := os.WriteFile(filePath, data, 0644)
		if err != nil {
			log.Fatalln(err)
		}
	}
	return file
}

func retrieveDbData(scanner *bufio.Scanner) map[string]string {
	db := make(map[string]string)

	for scanner.Scan() {
		line := scanner.Text()
		dbData := strings.Fields(line)

		if len(dbData) != 2 {
			log.Println("Skiping data")
		} else {
			db[dbData[0]] = dbData[1]
		}
	}
	return db
}

func initDbFile(filePath string) map[string]string {

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()

	clearDbForNewDay(scanner.Text(), filePath, file)

	db := retrieveDbData(scanner)

	return db
}

func retrieveFeeds(db map[string]string) map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fp := gofeed.NewParser()
	feed, _ := fp.ParseURLWithContext("https://azure.microsoft.com/en-us/blog/feed/", ctx)

	for _, item := range feed.Items {
		db[item.GUID] = item.Title
		fmt.Println(item.Link)
		fmt.Println("-------------------")
	}

	return db
}

func saveDb(db map[string]string) {
	file, err := os.OpenFile("db.txt", os.O_APPEND|os.O_WRONLY, 0644)

	if err != nil {
		log.Fatalln(err)
	}

	defer file.Close()

	for key, value := range db {
		tmp := fmt.Sprintf("%s %s\n", key, value)

		_, err := file.WriteString(tmp)

		if err != nil {
			log.Fatalln(err)
		}
	}
}

func main() {

	db := initDbFile("db.txt")
	db = retrieveFeeds(db)

	if db["123"] != "" {
		fmt.Println("found")
	}

	saveDb(db)
}
