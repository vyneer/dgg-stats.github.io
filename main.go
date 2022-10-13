package main

import (
	"bytes"
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

type Emote struct {
	Prefix string
}

type Link struct {
	Date time.Time
	Text string
}

type VyneerLog struct {
	Time     string
	Username string
	Message  string
}

var re_site *regexp.Regexp
var pattern string = `23\:[0-5][0-9]\:[0-5][0-9]`

// from here https://medium.com/@dhanushgopinath/concurrent-http-downloads-using-go-32fecfa1ed27
func downloadFile(client *http.Client, URL Link) ([]string, error) {
	var result []string
	req, _ := http.NewRequest("GET", string(URL.Text), nil)
	var response *http.Response
	var err error

	req, _ = http.NewRequest("GET", string(URL.Text), nil)
	response, err = client.Do(req)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		if response.StatusCode != http.StatusOK {
			switch response.StatusCode {
			case http.StatusBadRequest:
				return result, ErrBadRequest
			case http.StatusNotFound:
				return result, ErrNotFound
			case http.StatusForbidden:
				return result, ErrForbidden
			case http.StatusTooManyRequests:
				return result, ErrTooManyRequests
			case http.StatusInternalServerError:
				return result, ErrInternalServerError
			case http.StatusBadGateway:
				return result, ErrBadGateway
			case http.StatusServiceUnavailable:
				return result, ErrServiceUnavailable
			default:
				return result, errors.New(response.Status)
			}
		}
	}

	data := new(bytes.Buffer)
	_, err = io.Copy(data, response.Body)
	if err != nil {
		return result, err
	}

	// carriage returns break the bonus stats python script and could affect the pisg results
	noCR := strings.Replace(data.String(), "\r", "", -1)
	result = strings.Split(noCR, "\n")
	if re_site.FindAllString(result[len(result)-2], -1) == nil {
		fmt.Printf("%s seems a little sus, might be a broken file...\nHere's the last line for debugging reasons: %s\n", URL.Date, result[len(result)-2])
	}

	return result[0 : len(result)-1], nil
}

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

func getOverRustleURLs(from, to string) ([]Link, error) {
	startDate, _ := time.Parse("2006-01-02", from)
	endDate, _ := time.Parse("2006-01-02", to)
	var links []Link

	for rd := rangeDate(startDate, endDate); ; {
		date := rd()
		if date.IsZero() {
			break
		}
		links = append(links, Link{
			date,
			fmt.Sprintf("https://dgg.overrustlelogs.net/Destinygg chatlog/%s/%s.txt", date.Format("January 2006"), date.Format("2006-01-02")),
		})
	}
	var err error

	return links, err
}

func getDBLines(client *http.Client, from, to time.Time) ([]VyneerLog, error) {
	fromFormatted := from.Format("2006-01-02T15:04:05Z")
	toFormatted := to.Format("2006-01-02T15:04:05Z")

	url := fmt.Sprintf("https://vyneer.me/tools/rawlogs?from=%s&to=%s", fromFormatted, toFormatted)

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

// GetTextFiles downloads logs from OverRustleLogs
// starting and ending at specific timestamps
// and returns an array of chatlines.
func GetTextFiles(from, to, dir string) {
	links, _ := getOverRustleURLs(from, to)

	for _, link := range links {
		date := link.Date.Format("2006-01-02")
		pisglines := fmt.Sprintf("./cache/logs_%s_txt.pisglines", date)
		_, pisglinesExists := os.Stat(pisglines)
		pisgstats := fmt.Sprintf("./cache/logs_%s_txt.pisgstats", date)
		_, pisgstatsExists := os.Stat(pisgstats)
		if errors.Is(pisglinesExists, os.ErrNotExist) || errors.Is(pisgstatsExists, os.ErrNotExist) {
			fmt.Printf("Pulling %s...\n", date)
			var succ bool = false
			var vyneer bool = false
			var err error
			var result []string
			var finishedResult []string
			result, err = downloadFile(client, link)
			if err != nil {
				fmt.Printf("Got an error for %s: %s\n", date, err)
				switch {
				case errors.Is(err, ErrBadRequest), errors.Is(err, ErrNotFound),
					errors.Is(err, ErrForbidden), errors.Is(err, ErrTooManyRequests),
					errors.Is(err, ErrInternalServerError), errors.Is(err, ErrBadGateway),
					errors.Is(err, ErrServiceUnavailable), os.IsTimeout(err):
					fmt.Printf("Falling back to vyneer.me logs\n")
					succ = true
					vyneer = true
				default:
					for i := 0; i < 3; i++ {
						fmt.Printf("Retrying %s %d time...\n", date, i+1)
						fmt.Printf("10s timeout...\n")
						time.Sleep(time.Second * 10)
						result, err = downloadFile(client, link)
						if err == nil {
							fmt.Printf("We're good! Continuing...\n")
							succ = true
							break
						}
					}
				}
			} else {
				succ = true
			}

			if !succ {
				fmt.Printf("Skipping %s...\n", date)
				continue
			}

			if !vyneer {
				for _, line := range result {
					index := strings.Index(line, ": ")
					length := len(line)

					timestamp, _ := time.Parse("2006-01-02 15:04:05 UTC", line[1:24])
					timestamp1 := timestamp.Format("02/01/2006 @ 15:04:05")
					username := line[26:index]
					message := line[index+2 : length]
					finishedResult = append(finishedResult, fmt.Sprintf("[%s] <%s> %s", timestamp1, username, message))
				}
			} else {
				start := link.Date
				end := start.Add(24 * time.Hour)
				logs, err := getDBLines(client, start, end)
				if err != nil {
					fmt.Printf("[vyneer] Skipping %s...\n", date)
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
			}

			file, err := os.OpenFile(fmt.Sprintf("%s%s.txt", dir, date), os.O_CREATE|os.O_WRONLY, 0644)

			if err != nil {
				log.Fatalf("failed creating file: %s", err)
			}

			for _, data := range finishedResult {
				_, _ = file.WriteString(data + "\n")
			}

			file.Close()
		} else {
			fmt.Printf("Found %s in cache, skipping\n", date)
		}
	}
}

func SwapEmotes() {
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
	file, err := os.ReadFile("pisg.cfg.initial")
	if err != nil {
		log.Panicf("couldn't open pisg.cfg.initial, panicking")
	}
	fileString := string(file)
	replaced := strings.Replace(fileString, "ALOTOFEMOTES", resultingString, 1)
	newFile, err := os.OpenFile("pisg.cfg", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Panicf("couldn't create pisg.cfg, panicking")
	}
	_, err = newFile.WriteString(replaced)
	if err != nil {
		log.Panicf("couldn't write into pisg.cfg, panicking")
	}
}

func main() {
	re_site = regexp.MustCompile(pattern)
	args := os.Args[1:]

	SwapEmotes()
	GetTextFiles(args[0], args[1], args[2])
}
