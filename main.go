package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var transport http.RoundTripper = &http.Transport{
	DisableKeepAlives:  true,
	DisableCompression: true,
}

var client *http.Client = &http.Client{
	Transport: transport,
	Timeout:   30 * time.Second,
}

var ErrBadRequest = errors.New("400 Bad Request")
var ErrNotFound = errors.New("404 Not Found")
var ErrForbidden = errors.New("403 Forbidden")
var ErrTooManyRequests = errors.New("429 Too Many Requests")
var ErrInternalServerError = errors.New("500 Internal Server Error")
var ErrBadGateway = errors.New("502 Bad Gateway")
var ErrServiceUnavailable = errors.New("503 Service Unavailable")
var ErrMovedTemporarily = errors.New("302 Moved Temporarily")

type Emote struct {
	Prefix string
}

type VyneerLog struct {
	Time     string
	Username string
	Message  string
}

var re_site *regexp.Regexp
var pattern string = `23\:[0-5][0-9]\:[0-5][0-9]`

// rangeDate returns a date range function over start date to end date inclusive.
// After the end of the range, the range function returns a zero date,
// date.IsZero() is true.
func rangeDate(start, end time.Time) func() time.Time {
	y, m, d := start.Date()
	start = time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	y, m, d = end.Date()
	end = time.Date(y, m, d, 0, 0, 0, 0, time.UTC)

	return func() time.Time {
		if start.After(end) {
			return time.Time{}
		}
		date := start
		start = start.AddDate(0, 0, 1)
		return date
	}
}

func getDateSlice(from, to string) ([]time.Time, error) {
	startDate, _ := time.Parse("2006-01-02", from)
	endDate, _ := time.Parse("2006-01-02", to)
	var links []time.Time

	for rd := rangeDate(startDate, endDate); ; {
		date := rd()
		if date.IsZero() {
			break
		}
		links = append(links, date)
	}
	var err error

	return links, err
}

func getDBLines(client *http.Client, logsUrl string, from, to time.Time) ([]VyneerLog, error) {
	fromFormatted := from.Format("2006-01-02T15:04:05Z")
	toFormatted := to.Format("2006-01-02T15:04:05Z")

	url := fmt.Sprintf("%s?from=%s&to=%s", logsUrl, fromFormatted, toFormatted)

	req, _ := http.NewRequest("GET", url, nil)
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		switch response.StatusCode {
		case http.StatusBadRequest:
			return nil, ErrBadRequest
		case http.StatusNotFound:
			return nil, ErrNotFound
		case http.StatusForbidden:
			return nil, ErrForbidden
		case http.StatusTooManyRequests:
			return nil, ErrTooManyRequests
		case http.StatusInternalServerError:
			return nil, ErrInternalServerError
		case http.StatusBadGateway:
			return nil, ErrBadGateway
		case http.StatusServiceUnavailable:
			return nil, ErrServiceUnavailable
		default:
			return nil, errors.New(response.Status)
		}
	}
	b, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	var logs = []VyneerLog{}
	err = json.Unmarshal(b, &logs)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// GetTextFiles downloads logs
// starting and ending at specific timestamps
// and returns an array of chatlines.
func GetTextFiles(url, from, to, dir string) {
	dates, _ := getDateSlice(from, to)

	for _, start := range dates {
		//time.Sleep(5 * time.Second)
		formatted := start.Format("2006-01-02")
		fmt.Printf("Pulling %s...\n", formatted)
		var err error
		var finishedResult []string

		end := start.Add(24 * time.Hour)
		logs, err := getDBLines(client, url, start, end)
		if err != nil {
			fmt.Printf("[vyneer] Skipping %s...\n", formatted)
			continue
		}
		for _, line := range logs {
			timestampSplit := strings.SplitN(line.Time, "T", 2)
			timestampInit, _ := time.Parse("2006-01-02", timestampSplit[0])
			date := timestampInit.Format("02/01/2006")
			time := timestampSplit[1][:len(timestampSplit[1])-5]
			timestamp := fmt.Sprintf("%s @ %s", date, time)
			finishedResult = append(finishedResult, fmt.Sprintf("[%s] <%s> %s", timestamp, line.Username, line.Message))
		}

		file, err := os.OpenFile(fmt.Sprintf("%s%s.txt", dir, formatted), os.O_CREATE|os.O_WRONLY, 0644)

		if err != nil {
			log.Fatalf("failed creating file: %s", err)
		}

		for _, data := range finishedResult {
			_, _ = file.WriteString(data + "\n")
		}

		file.Close()
	}
}

func SwapEmotes(s string) string {
	req, _ := http.NewRequest("GET", "https://cdn.destiny.gg/emotes/emotes.json", nil)
	response, err := client.Do(req)
	if err != nil {
		log.Panicf("couldn't download emotes, panicking")
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		log.Panicf("couldn't download emotes, panicking")
	}

	b, err := io.ReadAll(response.Body)
	if err != nil {
		log.Panicf("couldn't download emotes, panicking")
	}

	var emotes = []Emote{}
	err = json.Unmarshal(b, &emotes)
	if err != nil {
		log.Panicf("couldn't unmarshal emotes, panicking")
	}

	var resultingString string
	for i, emote := range emotes {
		if i != len(emotes)-1 {
			resultingString += emote.Prefix + " "
		} else {
			resultingString += emote.Prefix
		}
	}

	replaced := strings.Replace(s, "ALOTOFEMOTES", resultingString, 1)

	return replaced
}

func GenerateIgnores(s string) string {
	file, err := os.ReadFile("top-250-words.txt")
	if err != nil {
		log.Panicf("couldn't open top-250-words.txt, panicking")
	}
	words := strings.Split(string(file), "\n")

	var resultingString string
	for i, word := range words {
		if i != len(words)-1 {
			resultingString += fmt.Sprintf("<user nick=\"%s\" refignore=\"y\">\n", word)
		} else {
			resultingString += fmt.Sprintf("<user nick=\"%s\" refignore=\"y\">", word)
		}
	}

	replaced := strings.Replace(s, "#REFIGNORE_REPLACE", resultingString, 1)

	return replaced
}

func GenerateConfig() {
	file, err := os.ReadFile("pisg.cfg.initial")
	if err != nil {
		log.Panicf("couldn't open pisg.cfg.initial, panicking")
	}
	fileString := string(file)

	fileString = SwapEmotes(fileString)
	fileString = GenerateIgnores(fileString)

	newFile, err := os.OpenFile("pisg.cfg", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Panicf("couldn't create pisg.cfg, panicking")
	}
	_, err = newFile.WriteString(fileString)
	if err != nil {
		log.Panicf("couldn't write into pisg.cfg, panicking")
	}
}

func main() {
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return ErrMovedTemporarily
	}
	re_site = regexp.MustCompile(pattern)
	args := os.Args[1:]

	if u, ok := os.LookupEnv("LOGS_URL"); ok {
		GenerateConfig()
		GetTextFiles(u, args[0], args[1], args[2])
	} else {
		log.Panic("no logs url provided")
	}
}
