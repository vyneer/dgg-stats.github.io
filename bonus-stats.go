package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"mvdan.cc/xurls/v2"
)

type kv struct {
	Key   string
	Value int
}

// from https://stackoverflow.com/questions/45030618/generate-a-random-bool-in-go
func RandBool() bool {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(2) == 1
}

func MakeHeader(title string) string {
	return fmt.Sprintf(`
	<table border="0" cellpadding="1" cellspacing="1" width="750">
	<tbody>
	<tr>
	<td class="headlinebg">
	<table border="0" cellpadding="2" cellspacing="0" width="100%%">
	<tbody>
	<tr>
	<td class="headtext">
	%s
	</td>
	</tr>
	</tbody>
	</table>
	</td>
	</tr>
	</tbody>
	</table>
	`, title)
}

func MakeLinkTable(first string, second string, third string, array1 []kv, array2 map[string]string) string {
	table := fmt.Sprintf(`
	<table border="0" width="754">
	<tbody>
	<tr>
	<td> </td>
	<td class="tdtop"><b>%s</b></td>
	<td class="tdtop"><b>%s</b></td>
	<td class="tdtop"><b>%s</b></td>
	</tr>
	`, first, second, third)
	for i, val := range array1 {
		table += MakeLinkEntry(i+1, val.Key, val.Value, array2[val.Key])
	}
	table += `
	</tbody>
	</table>
	`
	return table
}

func MakeLinkEntry(place int, nick string, count int, link string) string {
	if place == 1 {
		return fmt.Sprintf(`
		<tr>
		<td class="hirankc">%d</td>
		<td class="hicell">%s</td>
		<td class="hicell">%d</td>
		<td class="tdtop"><a href="%s" style="word-break: break-word">%s</a></td>
		</tr>
		`, place, nick, count, link, link)
	} else {
		return fmt.Sprintf(`
		<tr>
		<td class="rankc">%d</td>
		<td class="hicell">%s</td>
		<td class="hicell">%d</td>
		<td class="tdtop"><a href="%s" style="word-break: break-word">%s</a></td>
		</tr>
		`, place, nick, count, link, link)
	}
}

func belongsToBlacklist(lookup string) bool {
	switch lookup {
	case
		"Bot",
		"Logs",
		"RandomFerret",
		"SubscriberMessage":
		return true
	}
	return false
}

func main() {
	relaxed := xurls.Relaxed()

	coomerCount := make(map[string]int)
	coomerRand := make(map[string]string)

	linkCount := make(map[string]int)
	linkRand := make(map[string]string)

	var coomerCountSorted []kv
	var linkCountSorted []kv

	files, err := ioutil.ReadDir("logs")
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		file, err := ioutil.ReadFile(fmt.Sprintf("logs/%s", f.Name()))
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Processing %s...\n", f.Name())

		result := strings.Split(string(file), "\n")
		result = result[0 : len(result)-1]
		for _, chatLine := range result {
			split := strings.SplitN(chatLine[24:], " ", 2)

			username := strings.TrimPrefix(strings.TrimSuffix(split[0], ">"), "<")
			message := split[1]

			urls := relaxed.FindAllString(message, -1)

			if len(urls) != 0 {
				if !belongsToBlacklist(username) {
					if strings.Contains(strings.ToLower(message), "nsfw") {
						coomerCount[username] += 1
						if RandBool() {
							coomerRand[username] = urls[0]
						}
					}
					linkCount[username] += 1
					if RandBool() {
						linkRand[username] = urls[0]
					}
				}
			}
		}
	}

	for k, v := range coomerCount {
		coomerCountSorted = append(coomerCountSorted, kv{k, v})
	}

	sort.Slice(coomerCountSorted, func(i, j int) bool {
		return coomerCountSorted[i].Value > coomerCountSorted[j].Value
	})

	for k, v := range linkCount {
		linkCountSorted = append(linkCountSorted, kv{k, v})
	}

	sort.Slice(linkCountSorted, func(i, j int) bool {
		return linkCountSorted[i].Value > linkCountSorted[j].Value
	})

	f, err := os.Open("index.html")
	if err != nil {
		log.Fatal(err)
	}

	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("div[align=center]").Each(func(i int, div *goquery.Selection) {
		sel := div.Find(`table[width="750"][cellpadding="1"]`)
		for y := range sel.Nodes {
			if y == len(sel.Nodes)-1 {
				single := sel.Eq(y)
				single.BeforeHtml(fmt.Sprintf("%s%s<br>",
					MakeHeader("Biggest linkers"),
					MakeLinkTable("Nick", "Lines with Links", "Random Link", linkCountSorted[0:5], linkRand)))
				single.BeforeHtml(fmt.Sprintf("%s%s<br>",
					MakeHeader("Biggest coomers"),
					MakeLinkTable("Nick", "Lines with Coom Links", "Random Coom", coomerCountSorted[0:5], coomerRand)))
			}
		}
	})

	html, _ := doc.Html()
	ioutil.WriteFile("index.html", []byte(html), 0644)
}
