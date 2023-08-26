package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Segment struct {
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
	segments    []Segment
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
	file, err := os.Open(filename)

	if err != nil {
		fmt.Println("Couldn't read file SAD!")
		var source Source
		return source
	}

	scanner := bufio.NewScanner(file)

	var source Source
	var currentSegment Segment

	line_num := 0
	for scanner.Scan() {
		// first line is the number
		// second line is the timecodes
		// third line is the transcript
		// fourth line is etc...

		// fmt.Println(line_num, scanner.Text())
		line := scanner.Text()

		if line_num%4 == 0 {
			// skip
		} else if line_num%4 == 1 {
			halves := strings.Split(line, " --> ")

			// first time code
			hours, _ := strconv.Atoi(string(halves[0][0]) + string(halves[0][1]))
			minutes, _ := strconv.Atoi(string(halves[0][3]) + string(halves[0][4]))
			seconds, _ := strconv.Atoi(string(halves[0][6]) + string(halves[0][7]))

			currentSegment.start = uint32(seconds + minutes*60 + hours*60*60)

			hours, _ = strconv.Atoi(string(halves[1][0]) + string(halves[1][1]))
			minutes, _ = strconv.Atoi(string(halves[1][3]) + string(halves[1][4]))
			seconds, _ = strconv.Atoi(string(halves[1][6]) + string(halves[1][7]))
			currentSegment.end = uint32(seconds + minutes*60 + hours*60*60)
		} else if line_num%4 == 2 {
			currentSegment.text = line

			// append segment
			source.segments = append(source.segments, currentSegment)
		} else if line_num%4 == 3 {
			// skip
		}

		line_num++
	}

	return source
}

func reqAddYoutube(res http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	url := req.Form.Get("url")

	url = url[1:]
	url = url[:len(url)-1]

	// TODO: maybe validate input first
	res.Write([]byte("Yay"))

	parse := func() {
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

		files, err = ioutil.ReadDir(path)

		if err != nil {
			fmt.Println("Couldnt read path: " + path)
			return
		}

		for _, f := range files {
			// TODO: add actual group string
			fname := f.Name()
			extension := fname[len(fname)-4:]

			if extension == ".srt" {
				sources = append(sources, parseFile(path+"/"+f.Name(), ""))
			}
		}

		if len(sources) > 0 {
			fmt.Println(sources[len(sources)-1])
		}

		releaseTicket(ticket)
	}

	go parse()
}

func reqQuery(res http.ResponseWriter, req *http.Request) {
}

func main() {
	http.HandleFunc("/query", reqQuery)
	http.HandleFunc("/addyoutube", reqAddYoutube)

	fmt.Println("Started server")
	http.ListenAndServe(":8888", nil)
}
