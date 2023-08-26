package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"os/exec"
)

type Section struct {
	start uint32
	end   uint32
	text  string
}

type Source struct {
	name        string
	description string
	url         string
	subtitles   string
}

var sources []Source
var usedTickets [][32]byte

func getTicket() [32]byte {
	alphabet := []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 't', 's', 'u', 'v', 'w', 'x', 'y', 'z', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}

	unique := false

	// generate a unique ticket
	var ticket [32]byte
	for !unique {
		unique = true

		for i := range ticket {
			ticket[i] = alphabet[rand.Intn(len(alphabet))]
		}

		for i := range usedTickets {
			sameTicket := true
			for j := range ticket {
				if usedTickets[i][j] != ticket[j] {
					sameTicket = false
				}
			}

			if sameTicket {
				unique = false
				break
			}
		}
	}

	usedTickets = append(usedTickets, ticket)

	return ticket
}

func releaseTicket(ticket [32]byte) {
	for i := range usedTickets {
		sameTicket := true
		for j := range ticket {
			if usedTickets[i][j] != ticket[j] {
				sameTicket = false
			}
		}

		if sameTicket {
			// remove
			usedTickets[i] = usedTickets[len(usedTickets)-1]
			usedTickets = usedTickets[:len(usedTickets)-1]
		}
	}
}

func downloadYoutube(ticket string, url string) {
	// name should just be url
	// IF THERE'S ANY PROBLEM IT'S PROBABLY THE -F 250 part
	exec.Command("yt-dlp", "-o", "downloaded/%(title)s|https:~~youtube.com~watch?v=%(id)s", "-F", "250", "-x", "--audio-format", "wav", url)

	fmt.Println("Finished downloading")
	// move to url name or something

	// parse titles and stuff
}

func downloadPodcast() {
	// similar idea
}

func transcribe() {
}

func parseFile() {
}

func reqAddYoutube(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	url := req.Form.Get("url")

	fmt.Println("Downloading " + url)
	ticket := getTicket()
	downloadYoutube(ticket, url)

	res.Write([]byte("Yay"))
}

func reqQuery(res http.ResponseWriter, req *http.Request) {
}

func main() {
	http.HandleFunc("/query", reqQuery)
	http.HandleFunc("/addyoutube", reqAddYoutube)

	fmt.Println("Started server")
	http.ListenAndServe(":8888", nil)
}
