package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
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

func sendToDiscord(db map[string]FeedInfo) {

	webhookUrl := os.Getenv("WEBHOOK")

	for _, value := range db {
		if value.Link != "" {
			content := fmt.Sprintf("{\"content\": \"[%s](%s)\"}", value.Title, value.Link)
			var jsonData = []byte(content)

			request, _ := http.NewRequest("POST", webhookUrl, bytes.NewBuffer(jsonData))
			request.Header.Set("Content-Type", "application/json")

			client := &http.Client{}
			response, err := client.Do(request)
			if err != nil {
				log.Fatalln(err)
			}

			if response.StatusCode != 204 {
				log.Println("response Status:", response.StatusCode)
			}

			response.Body.Close()
			log.Println(value.Title)
		}
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

	sendToDiscord(db)

	saveDb(db)
}
