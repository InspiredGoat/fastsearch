package main

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rs/cors"
)

type Segment struct {
	Start uint32
	End   uint32
	Text  string
}

type Result struct {
	text   string
	start  uint32
	end    uint32
	source *Source
}

type Source struct {
	Name string
	// so if they come from a single playlist or channel queue, that sort of thing
	Group       string
	Description string
	Author      string
	Url         string
	Segments    []Segment
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
	filename := path + "/%(title)s{{%(id)s}}"

	fmt.Println("yt-dlp", "-o", "'"+filename+"'", "-x", "--audio-format", "wav", url)
	cmd := exec.Command("yt-dlp", "-o", filename, "-x", "--audio-format", "wav", url)

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
}

// https://www.youtube.com/watch?v=3a8jlMvBCGY
// https://www.youtube.com/watch?v=3a8jlMvBCGY

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

			currentSegment.Start = uint32(seconds + minutes*60 + hours*60*60)

			hours, _ = strconv.Atoi(string(halves[1][0]) + string(halves[1][1]))
			minutes, _ = strconv.Atoi(string(halves[1][3]) + string(halves[1][4]))
			seconds, _ = strconv.Atoi(string(halves[1][6]) + string(halves[1][7]))
			currentSegment.End = uint32(seconds + minutes*60 + hours*60*60)
		} else if line_num%4 == 2 {
			currentSegment.Text = strings.ToLower(line)

			// append segment
			source.Segments = append(source.Segments, currentSegment)
		} else if line_num%4 == 3 {
			// skip
		}

		line_num++
	}

	return source
}

func reqAddYoutube(w http.ResponseWriter, req *http.Request) {

	req.ParseForm()
	url := req.Form.Get("url")

	url = url[1:]
	url = url[:len(url)-1]

	// TODO: maybe validate input first
	w.Write([]byte("Yay"))

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
		fmt.Println("Done transcibing")

		files, err = ioutil.ReadDir(path)

		if err != nil {
			fmt.Println("Couldnt read path: " + path)
			return
		}

		fmt.Println("Parsing")
		for _, f := range files {
			// TODO: add actual group string
			fname := f.Name()
			extension := fname[len(fname)-4:]

			if extension == ".srt" {
				s := parseFile(path+"/"+f.Name(), "")
				// video title ideally
				s.Name = f.Name()
				s.Group = url
				id := strings.Split(f.Name(), "{{")[1]
				id = strings.Split(id, "}}")[0]
				s.Url = "https://www.youtube.com/watch?v=" + id
				fmt.Println(s.Url)
				sources = append(sources, s)
			}
		}
		fmt.Println("Done parsing")

		if len(sources) > 0 {
			fmt.Println(sources[len(sources)-1])
		}

		releaseTicket(ticket)
		fmt.Println("Saving")
		saveSources()
	}

	go parse()
}

func reqQuery(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	q := strings.ToLower(req.Form.Get("search"))

	var top_results []Result

	for i := range sources {
		s := &sources[i]

		// first exact matches
		for j := range s.Segments {
			var res Result
			res.text = ""
			res.start = s.Segments[j].Start
			res.end = s.Segments[j].End
			res.source = s

			// if j > 0 {
			// 	res.text += s.Segments[j-1].Text
			// 	res.start = s.Segments[j].Start
			// }

			res.text += s.Segments[j].Text

			// if j < len(s.Segments)-1 {
			// 	res.text += s.Segments[j+1].Text
			// 	res.end = s.Segments[j].End
			// }

			if strings.Contains(res.text, q) {
				top_results = append(top_results, res)
			}
		}
	}

	fmt.Fprintln(w, "<ul>")
	for i, r := range top_results {
		if i > 10 {
			break
		}

		fmt.Fprintln(w, "<li>")
		fmt.Fprintln(w, r.text)
		fmt.Fprintln(w, "</li>")
		fmt.Fprintln(w, "<iframe src=\"")
		fmt.Fprintln(w, r.source.Url)
		fmt.Fprintln(w, "\">")
		fmt.Fprintln(w, "</iframe>")
	}
	fmt.Fprintln(w, "</ul>")
}

func reqDummyQuery(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()

	fmt.Println("thsi ran")
	q := req.Form.Get("search")

	fmt.Fprintln(w, "<ul>")
	fmt.Fprintln(w, "<li>\"fjawoi oiwea <b>"+q+"</b> fjowei jfioaew\"</li>")
	fmt.Fprintln(w, "<li>\"oiwea <b>"+q+"</b> jfioaew\"</li>")
	fmt.Fprintln(w, "<li>\"fjawoi oiwea <b>"+q+"</b> ffjaewiof jaweoifjowei jfioaew\"</li>")
	fmt.Fprintln(w, "</ul>")
}

func saveSources() {
	for _, s := range sources {
		f, err := os.Create("savedsources/" + s.Name + ".bin")

		if err != nil {
			fmt.Println("Error opening: " + s.Name + ".bin")
		} else {
			defer f.Close()

			enc := gob.NewEncoder(f)
			fmt.Println("encoding", s)
			err := enc.Encode(s)
			if err != nil {
				fmt.Println("Error encoding: " + s.Name)
				fmt.Println(err.Error())
			} else {
			}
		}
	}
}

func loadSources() {
	files, err := ioutil.ReadDir("savedsources/")
	if err != nil {
		fmt.Println("Couldn't open sources directory")
	}

	for _, f := range files {
		var s Source
		fname := f.Name()
		f, err := os.Open("savedsources/" + f.Name())
		fmt.Println("Trying to decode: savedsources/" + fname)

		if err != nil {
			fmt.Println("Error opening: savedsources" + fname + ".bin")
			fmt.Println(err.Error())
		} else {
			defer f.Close()

			dec := gob.NewDecoder(f)
			err := dec.Decode(&s)
			if err != nil {
				fmt.Println("Error decoding: " + f.Name())
				fmt.Println(err.Error())
			} else {
				sources = append(sources, s)
			}
		}
	}
}

func main() {
	loadSources()

	mux := http.NewServeMux()
	mux.HandleFunc("/query", reqQuery)
	mux.HandleFunc("/addyoutube", reqAddYoutube)
	mux.HandleFunc("/trolling", func(w http.ResponseWriter, req *http.Request) {
		fmt.Println("trolled!")
		fmt.Fprintln(w, "Yo wassup homie")
	})

	fmt.Println("Started server")

	handler := cors.AllowAll().Handler(mux)
	http.ListenAndServe(":8888", handler)
}
