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
	Id          string
	Segments    []Segment
}

type Ticket struct {
	Stage    string
	Progress int
	id       [32]byte
}

var sources []Source
var usedTickets []Ticket

func getTicket() *Ticket {
	alphabet := []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 't', 's', 'u', 'v', 'w', 'x', 'y', 'z', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}

	unique := false

	// generate a unique ticket_id
	var ticket_id [32]byte
	for !unique {
		unique = true

		for i := range ticket_id {
			ticket_id[i] = alphabet[rand.Intn(len(alphabet))]
		}

		for i := range usedTickets {
			sameTicket := true
			for j := range ticket_id {
				if usedTickets[i].id[j] != ticket_id[j] {
					sameTicket = false
				}
			}

			if sameTicket {
				unique = false
				break
			}
		}
	}

	var ticket Ticket
	ticket.id = ticket_id
	usedTickets = append(usedTickets, ticket)

	return &usedTickets[len(usedTickets)-1]
}

func releaseTicket(ticket Ticket) {
	for i := range usedTickets {
		sameTicket := true
		for j := range ticket.id {
			if usedTickets[i].id[j] != ticket.id[j] {
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
			currentSegment.Text = line

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

		path := "downloaded/" + string(ticket.id[:])
		ticket.Stage = "Downloading..."
		ticket.Progress = 0
		os.MkdirAll(path, 0o777)
		fmt.Println("Downloading " + url)
		downloadYoutube(path, url)
		fmt.Println("done downloading")

		files, err := ioutil.ReadDir(path)

		if err != nil {
			fmt.Println("Couldnt read path: " + path)
			return
		}

		ticket.Stage = "Transcribing..."
		ticket.Progress = 50
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

		ticket.Stage = "Transcribing..."
		ticket.Progress = 90
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
				s.Id = id
				s.Url = "https://www.youtube.com/watch?v=" + id
				fmt.Println(s.Url)
				sources = append(sources, s)
			}
		}
		fmt.Println("Done parsing")

		ticket.Stage = "Saving..."
		ticket.Progress = 90
		if len(sources) > 0 {
			fmt.Println(sources[len(sources)-1])
		}

		releaseTicket(*ticket)
		fmt.Println("Saving")
		saveSources()
	}

	go parse()
}

func reqQuery(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	q := strings.ToLower(req.Form.Get("search"))

	if q == "" {
		return
	}

	var top_results []Result

	for i := range sources {
		s := &sources[i]

		if req.Form.Get(s.Name) == "enabled" {
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

				if strings.Contains(strings.ToLower(res.text), q) {
					top_results = append(top_results, res)
				}
			}
		}
	}

	fmt.Fprintln(w, "<ul>")
	for i, r := range top_results {
		if i > 10 {
			break
		}

		fmt.Fprintln(w, "<li>")
		i := strings.Index(r.text, q)

		fmt.Fprint(w, "“")
		text := strings.TrimSpace(r.text)
		if i != -1 {
			fmt.Fprint(w, text[:i-1])
			fmt.Fprint(w, "<b>")
			fmt.Fprint(w, text[i-1:i-1+len(q)])
			fmt.Fprint(w, "</b>")
			fmt.Fprint(w, text[i-1+len(q):])
			fmt.Fprint(w)
		} else {
			fmt.Fprint(w, text)
		}
		fmt.Fprintln(w, "”")

		fmt.Fprintln(w, "<a href=\"")
		fmt.Fprintln(w, r.source.Url+"&t="+strconv.Itoa(int(r.start)))
		fmt.Fprintln(w, "\">")
		fmt.Fprintln(w, "⮫")
		fmt.Fprintln(w, "</a>")
		fmt.Fprintln(w, "</li>")

		fmt.Fprintln(w, "<a href=\"#\" onclick=\"toggleChildVisible(event)\">🠊")
		fmt.Fprintln(w, "<div class=\"hidden\">")
		fmt.Fprintln(w, "<iframe class=\"ytembed\" src=\"")
		fmt.Fprintln(w, "https://www.youtube.com/embed/"+r.source.Id+"?start="+strconv.Itoa(int(r.start)))
		fmt.Fprintln(w, "\" title=\"YouTube video player\" frameborder=\"0\" allow=\"accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share\" allowfullscreen>")
		fmt.Fprintln(w, "</iframe>")
		fmt.Fprintln(w, "</div>")
		fmt.Fprintln(w, "</a>")
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

func reqGetSources(w http.ResponseWriter, req *http.Request) {
	for _, s := range sources {
		fmt.Fprintln(w, "<input id=\""+s.Name+"\" name=\""+s.Name+"\" class=\"source-checkbox\" value=\"enabled\" type=\"checkbox\" onclick=\"htmx.trigger('#search', 'search')\" checked>")
		fmt.Fprintln(w, "<label for=\""+s.Name+"\">"+s.Name+"</label><br><br>")
	}
}

func main() {
	loadSources()

	mux := http.NewServeMux()
	mux.HandleFunc("/query", reqQuery)
	mux.HandleFunc("/addyoutube", reqAddYoutube)
	mux.HandleFunc("/getsources", reqGetSources)
	mux.HandleFunc("/trolling", func(w http.ResponseWriter, req *http.Request) {
		fmt.Println("trolled!")
		fmt.Fprintln(w, "Yo wassup homie")
	})

	fmt.Println("Started server")

	handler := cors.AllowAll().Handler(mux)
	http.ListenAndServe(":8888", handler)
}
