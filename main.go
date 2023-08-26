package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
)

type Section struct {
	start uint32
	end   uint32
	text  string
}

type Source struct {
	name string
	// so if they come from a single playlist or channel queue, that sort of thing
	group       string
	description string
	author      string
	url         string
	subtitles   []Section
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

// returns filename
func downloadYoutube(path string, url string) {
	// name should just be url
	// IF THERE'S ANY PROBLEM IT'S PROBABLY THE -F 250 part
	filename := path + "/%(id)s"
	fmt.Println("yt-dlp", "-o", "'"+filename+"'", "-x", "--audio-format", "wav", url)
	cmd := exec.Command("yt-dlp", "-o", filename, "-f", "250", "-x", "--audio-format", "wav", url)

	cmd.Wait()
	out, err := cmd.Output()
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Printf("%s\n", out)
}

func downloadPodcast() {
	// similar idea but parse RSS feed first and stuff
}

func transcribe(filename string) {
	fmt.Println("Started transcibing: " + filename)
	cmd := exec.Command("shisper", "transcribe", filename)
	cmd.Wait()
	out, err := cmd.Output()
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Printf("%s\n", out)
	fmt.Println("Done transcibing")
}

func parseFile(filename string, group string) Source {
	buf, _ := ioutil.ReadFile(filename)

	// first line is error
}

func reqAddYoutube(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	url := req.Form.Get("url")

	url = url[1:]
	url = url[:len(url)-1]

	ticket := getTicket()

	path := "downloaded/" + string(ticket[:])
	os.MkdirAll(path, 0o777)
	fmt.Println("Downloading " + url)
	downloadYoutube(path, url)
	fmt.Println("done downloading")

	files, err := ioutil.ReadDir(path)

	if err != nil {
		fmt.Println("Couldnt read path: " + path)
		return
	}

	for _, f := range files {
		fmt.Println("transcribing " + f.Name())
		transcribe(path + "/" + f.Name())
	}

	releaseTicket(ticket)
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
