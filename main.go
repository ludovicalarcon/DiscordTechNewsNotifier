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
const logFilePath = "logs.txt"
const dateLayout = "2006-01-02"

var currentDate = time.Now().UTC()

type FeedInfo struct {
	Title     string
	Link      string
	Published time.Time
}

func isFromMoreThanSevenDays(date time.Time) bool {
	sevenDayBefore := currentDate.AddDate(0, 0, -7)

	if date.Before(currentDate.Add(24*time.Hour)) && date.After(sevenDayBefore) {
		return false
	}
	return true
}

func retrieveDbData(scanner *bufio.Scanner) map[string]FeedInfo {
	db := make(map[string]FeedInfo)

	for scanner.Scan() {
		line := scanner.Text()
		dbData := strings.Split(line, "|-|")

		if len(dbData) != 3 {
			log.Println("Skiping data")
		} else {
			parsedDate, err := time.Parse(dateLayout, dbData[2])
			if err != nil {
				log.Fatalln(err)
			}
			if isFromMoreThanSevenDays(parsedDate) {
				log.Println(dbData[1], "is too old... discarding")
			} else {
				db[dbData[0]] = FeedInfo{Title: dbData[1], Published: parsedDate}
			}
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
		if !isFromMoreThanSevenDays(item.PublishedParsed.UTC()) && db[item.GUID] == (FeedInfo{}) {
			db[item.GUID] = FeedInfo{Title: item.Title, Link: item.Link, Published: item.PublishedParsed.UTC()}
		}
	}
	return db
}

func retrieveFeedsFromSources(db map[string]FeedInfo) map[string]FeedInfo {
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
			if webhookUrl != "" {
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
					log.Println("ERROR: response Status:", response.StatusCode)
				}

				response.Body.Close()
				log.Println(value.Title)
				time.Sleep(10 * time.Second)
			} else {
				log.Println(value.Title, value.Link, value.Published)
			}
		}
	}
}

func saveDb(db map[string]FeedInfo) {
	file, err := os.OpenFile(dbPath, os.O_WRONLY, 0644)

	if err != nil {
		log.Fatalln(err)
	}

	defer file.Close()

	err = os.WriteFile(dbPath, []byte(""), 0644)
	if err != nil {
		log.Fatalln(err)
	}

	for key, value := range db {
		tmp := fmt.Sprintf("%s|-|%s|-|%s\n", key, value.Title, value.Published.Format(dateLayout))

		_, err := file.WriteString(tmp)

		if err != nil {
			log.Fatalln(err)
		}
	}
}

func main() {
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)

	if err != nil {
		log.Fatalln(err)
	}
	defer logFile.Close()

	log.SetOutput(logFile)

	db := initDbFile(dbPath)
	db = retrieveFeedsFromSources(db)

	sendToDiscord(db)

	saveDb(db)
}
