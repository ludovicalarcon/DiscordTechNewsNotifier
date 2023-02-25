package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
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
				log.Println(dbData[1], "is too old... discarding - ", parsedDate)
			} else {
				log.Println("Retrieve from db", dbData[1], " - ", parsedDate.Format(dateLayout))
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

	db := retrieveDbData(scanner)

	return db
}

func retrieveFeeds(db map[string]FeedInfo, feedUrl string, currentDate time.Time) map[string]FeedInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(feedUrl, ctx)

	if err != nil {
		msg := fmt.Sprintf("Could not retrieve feed %s", feedUrl)
		date := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC)
		log.Printf("%s: %v", msg, err)
		db["ERROR"] = FeedInfo{Title: msg, Link: feedUrl, Published: date}
	} else {

		for _, item := range feed.Items {
			published := item.PublishedParsed.UTC()
			date := time.Date(published.Year(), published.Month(), published.Day(), 0, 0, 0, 0, time.UTC)
			if !isFromMoreThanSevenDays(date) && db[item.GUID] == (FeedInfo{}) {
				log.Println("Add to db", item.Title, " - ", date)
				db[item.GUID] = FeedInfo{Title: item.Title, Link: item.Link, Published: date}
			}
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
		time.Sleep(2 * time.Second)
	}

	return db
}

func sendToDiscord(db map[string]FeedInfo) {

	webhookUrl := os.Getenv("WEBHOOK")

	for _, value := range db {
		if value.Link != "" {
			if webhookUrl != "" {
				content := fmt.Sprintf("{\"content\": \"[%s](%s) - %s\"}", value.Title, value.Link, value.Published)
				var jsonData = []byte(content)

				request, _ := http.NewRequest("POST", webhookUrl, bytes.NewBuffer(jsonData))
				request.Header.Set("Content-Type", "application/json")

				client := &http.Client{}
				response, err := client.Do(request)
				if err != nil {
					log.Println("ERROR: could not send to discord", err)
				} else {

					if response.StatusCode != 204 {
						log.Println("ERROR: response Status:", response.StatusCode)
					}

					response.Body.Close()
					log.Println(value.Title)
					time.Sleep(2 * time.Second)
				}
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

		log.Println("Saving to db:", value.Title, value.Published.Format(dateLayout))
	}
}

func main() {
	debugMode, _ := strconv.ParseBool(os.Getenv("DEBUG"))

	if debugMode {
		logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)

		if err != nil {
			log.Fatalln(err)
		}
		defer logFile.Close()

		log.SetOutput(logFile)
	}

	db := initDbFile(dbPath)
	db = retrieveFeedsFromSources(db)

	sendToDiscord(db)

	saveDb(db)
	log.Println("------------------------------------------")
}
