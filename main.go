package main

import (
	"bytes"
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

type Link struct {
	Date string
	Text string
}

var re_site *regexp.Regexp
var pattern string = `23\:[0-5][0-9]\:[0-5][0-9]`

//from here https://medium.com/@dhanushgopinath/concurrent-http-downloads-using-go-32fecfa1ed27
func downloadFile(client *http.Client, URL Link) ([]string, error) {
	var result []string
	req, _ := http.NewRequest("GET", string(URL.Text), nil)
	var response *http.Response
	var err error
	// we're gonna send the request twice bc for whatever reason we dont get content-length the first time
	for i := 0; i < 2; i++ {
		response, err = client.Do(req)
		if err != nil {
			return result, err
		}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result, errors.New(response.Status)
	}

	req, _ = http.NewRequest("GET", string(URL.Text), nil)
	response, err = client.Do(req)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result, errors.New(response.Status)
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
	if response.ContentLength == int64(data.Len()) {
		return result[0 : len(result)-1], nil
	} else {
		return result[0 : len(result)-1], errors.New("log file doesnt end properly")
	}
	//fmt.Println(noCR[len(noCR)-100 : len(noCR)-1])

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
			date.Format("2006-01-02"),
			fmt.Sprintf("https://dgg.overrustlelogs.net/Destinygg chatlog/%s/%s.txt", date.Format("January 2006"), date.Format("2006-01-02")),
		})
	}
	var err error

	return links, err
}

// GetTextFiles downloads logs from OverRustleLogs
// starting and ending at specific timestamps
// and returns an array of chatlines.
func GetTextFiles(from, to, dir string) {
	links, _ := getOverRustleURLs(from, to)

	var transport http.RoundTripper = &http.Transport{
		DisableKeepAlives:  true,
		DisableCompression: true,
	}

	client := &http.Client{Transport: transport}

	for _, link := range links {
		//time.Sleep(5 * time.Second)
		fmt.Printf("Pulling %s...\n", link.Date)
		var succ bool = false
		var err error
		var result []string
		var finishedResult []string
		result, err = downloadFile(client, link)
		if err != nil {
			fmt.Printf("Got an error for %s...\n", link.Date)
			for i := 0; i < 3; i++ {
				fmt.Printf("Retrying %s %d time...\n", link.Date, i+1)
				fmt.Printf("10s timeout...\n")
				time.Sleep(time.Second * 10)
				result, err = downloadFile(client, link)
				if err == nil {
					fmt.Printf("We're good! Continuing...\n")
					succ = true
					break
				}
			}
		} else {
			succ = true
		}

		if !succ {
			fmt.Printf("Skipping %s...\n", link.Date)
			continue
		}

		for _, line := range result {
			index := strings.Index(line, ": ")
			length := len(line)

			timestamp, _ := time.Parse("2006-01-02 15:04:05 UTC", line[1:24])
			timestamp1 := timestamp.Format("02/01/2006 @ 15:04:05")
			username := line[26:index]
			message := line[index+2 : length]
			finishedResult = append(finishedResult, fmt.Sprintf("[%s] <%s> %s", timestamp1, username, message))
		}

		file, err := os.OpenFile(fmt.Sprintf("%s%s.txt", dir, link.Date), os.O_CREATE|os.O_WRONLY, 0644)

		if err != nil {
			log.Fatalf("failed creating file: %s", err)
		}

		for _, data := range finishedResult {
			_, _ = file.WriteString(data + "\n")
		}

		file.Close()
	}
}

func main() {
	re_site = regexp.MustCompile(pattern)
	args := os.Args[1:]

	GetTextFiles(args[0], args[1], args[2])
}
