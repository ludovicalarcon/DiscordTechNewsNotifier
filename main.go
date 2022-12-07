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

const sourcesPath = "sources.txt"
const dbPath = "db.txt"

type FeedInfo struct {
	Title string
	Link  string
}

func isSameDay(date1 time.Time, date2 time.Time) bool {
	if date1.Year() == date2.Year() && date1.Month() == date2.Month() && date1.Day() == date2.Day() {
		return true
	}
	return false
}

func clearDbForNewDay(firstLine string, filePath string, file *os.File) *os.File {
	const dateLayout = "2006-01-02"

	currentDate := time.Now().UTC()
	isNeededToClear := true

	if firstLine != "" {
		dbDate, err := time.Parse(dateLayout, firstLine)
		if err != nil {
			log.Fatalln(err)
		}

		if isSameDay(currentDate, dbDate) {
			isNeededToClear = false
		}
	}

	if isNeededToClear {
		log.Println("Clear db")
		data := []byte(fmt.Sprintf("%v\n", currentDate.Format(dateLayout)))
		err := os.WriteFile(filePath, data, 0644)
		if err != nil {
			log.Fatalln(err)
		}
	}
	return file
}

func retrieveDbData(scanner *bufio.Scanner) map[string]FeedInfo {
	db := make(map[string]FeedInfo)

	for scanner.Scan() {
		line := scanner.Text()
		dbData := strings.Split(line, "|-|")

		if len(dbData) != 2 {
			log.Println("Skiping data")
		} else {
			db[dbData[0]] = FeedInfo{Title: dbData[1]}
		}
	}
	return db
}

func initDbFile(filePath string) map[string]FeedInfo {

	file, err := os.OpenFile(filePath, os.O_CREATE, 0644)
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

func retrieveFeeds(db map[string]FeedInfo, feedUrl string, currentDate time.Time) map[string]FeedInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(feedUrl, ctx)

	if err != nil {
		log.Fatalln(err)
	}

	for _, item := range feed.Items {
		if isSameDay(currentDate, item.PublishedParsed.UTC()) && db[item.GUID] == (FeedInfo{}) {
			db[item.GUID] = FeedInfo{Title: item.Title, Link: item.Link}
		}
	}
	return db
}

func retrieveFeedsFromSources(db map[string]FeedInfo) map[string]FeedInfo {
	currentDate := time.Now().UTC()

	file, err := os.Open(sourcesPath)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		url := scanner.Text()
		db = retrieveFeeds(db, url, currentDate)
	}

	return db
}

func send(db map[string]FeedInfo) {
	for key, value := range db {
		fmt.Printf("%s %s %s\n", key, value.Title, value.Link)
	}
}

func saveDb(db map[string]FeedInfo) {
	file, err := os.OpenFile(dbPath, os.O_APPEND|os.O_WRONLY, 0644)

	if err != nil {
		log.Fatalln(err)
	}

	defer file.Close()

	for key, value := range db {
		if value.Link != "" {
			tmp := fmt.Sprintf("%s|-|%s\n", key, value.Title)

			_, err := file.WriteString(tmp)

			if err != nil {
				log.Fatalln(err)
			}
		}
	}
}

func main() {

	db := initDbFile(dbPath)
	db = retrieveFeedsFromSources(db)

	send(db)

	saveDb(db)
}
