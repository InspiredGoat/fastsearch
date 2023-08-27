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
	name     string
	Stage    string
	Progress int
	id       [32]byte
}

var sources []Source
var usedTickets []Ticket

func makeId() string {
	alphabet := []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 't', 's', 'u', 'v', 'w', 'x', 'y', 'z', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9'}

	unique := false

	// generate a unique ticket_id
	var ticket_id [64]byte
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

	return string(ticket_id[:])
}

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
func getVideoOrPlaylistName(url string) string {
	cmd := exec.Command("yt-dlp", "--print", "playlist", url)
	cmd.Wait()
	out, _ := cmd.Output()

	if !strings.Contains(string(out[:]), "NA") {
		return strings.Split(string(out[:]), "\n")[0]
	} else {
		cmd := exec.Command("yt-dlp", "--print", "title", url)
		cmd.Wait()

		out, _ = cmd.Output()
		return strings.Split(string(out[:]), "\n")[0]
	}
}
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

	// TODO: maybe validate input first
	w.Write([]byte("Yay"))

	parse := func() {
		ticket := getTicket()

		ticket.Stage = "Finding name..."
		ticket.name = getVideoOrPlaylistName(url)
		ticket.Progress = 10

		path := "downloaded/" + string(ticket.id[:])
		ticket.Stage = "Downloading..."
		os.MkdirAll(path, 0o777)
		fmt.Println("Downloading " + url)
		downloadYoutube(path, url)
		fmt.Println("done downloading")
		ticket.Progress = 30

		files, err := ioutil.ReadDir(path)

		if err != nil {
			fmt.Println("Couldnt read path: " + path)
			return
		}

		ticket.Stage = "Transcribing..."
		for _, f := range files {
			fmt.Println("transcribing " + f.Name())
			transcribe(path + "/" + f.Name())
			ticket.Progress += (80 - 30) / len(files)
		}
		ticket.Progress = 80
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
				s.Name = strings.Split(f.Name(), "{{")[0]
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
		}

		releaseTicket(*ticket)
		fmt.Println("Saving")
		saveSources()
	}

	go parse()
}

func levenshtein(a string, b string, n int, m int) int {
	if n == 0 {
		return m
	} else if m == 0
		return n
	} else if a[n - 1] == b[m - 1] {
	} else {
		min
		lev(a, b, n - 1, m - 1), lev()
	}

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

	fmt.Fprintln(w, "<p> Found", len(top_results), "matches</p>")
	fmt.Fprintln(w, "<ul>")

	for i, r := range top_results {
		if i > 10 {
			break
		}

		fmt.Fprintln(w, "<li>")
		i := strings.Index(strings.ToLower(r.text), q)
		minutes := int(r.start / 60)
		seconds := int(int(r.start) - minutes*60)
		fmt.Fprintln(w, "<p><u>", r.source.Name+"</u>", "at", strconv.Itoa(minutes)+"m"+strconv.Itoa(seconds)+"s", "</p>")

		id := makeId()
		id2 := makeId()
		fmt.Fprintln(w, "<a href=\"#\" onclick=\"toggleIdVisible('"+id+"');toggleIdClass('"+id2+"', 'spun')\">")

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

		fmt.Fprintln(w, "<i id=\""+id2+"\"class=\"fa fa-caret-right\" aria-hidden=\"true\"></i></a>")

		fmt.Fprintln(w, "<a href=\"")
		fmt.Fprintln(w, r.source.Url+"&t="+strconv.Itoa(int(r.start)))
		fmt.Fprintln(w, "\">")
		fmt.Fprintln(w, "<i class=\"fa fa-external-link\" aria-hidden=\"true\"></i>")
		fmt.Fprintln(w, "</a>")
		fmt.Fprintln(w, "</li>")

		fmt.Fprintln(w, "<div class=\"hidden\" id=\""+id+"\">")
		fmt.Fprintln(w, "<iframe class=\"ytembed\" src=\"")
		fmt.Fprintln(w, "https://www.youtube.com/embed/"+r.source.Id+"?start="+strconv.Itoa(int(r.start)))
		fmt.Fprintln(w, "\" title=\"YouTube video player\" frameborder=\"0\" allow=\"accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share\" allowfullscreen>")
		fmt.Fprintln(w, "</iframe>")
		fmt.Fprintln(w, "</div>")
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
func reqGetRawSources(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintln(w, "<ul>")
	for _, s := range sources {
		fmt.Fprintln(w, "<li>"+s.Name+"</li>")
	}
	fmt.Fprintln(w, "</ul>")
}
func reqGetTickets(w http.ResponseWriter, req *http.Request) {
	if len(usedTickets) > 0 {
		fmt.Fprintln(w, "<h2>Downloading videos</h2>")
		fmt.Fprintln(w, "<ul>")
		for _, t := range usedTickets {
			fmt.Fprintln(w, "<li><div class=\"progress\">"+t.name, t.Stage)
			fmt.Fprintln(w, "<div class=\"progress-inside\" width=\"", t.Progress, "%\"")
			fmt.Fprintln(w, "background-color=\"")
			if t.Progress < 30 {
				fmt.Fprintln(w, "red")
			} else if t.Progress < 50 {
				fmt.Fprintln(w, "orange")
			} else if t.Progress < 70 {
				fmt.Fprintln(w, "light-green")
			} else {
				fmt.Fprintln(w, "green")
			}
			fmt.Fprintln(w, "\"></div></div></li>")
		}
		fmt.Fprintln(w, "</ul>")
	}
}

func reqSourceModal(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintln(w, "<div id=\"modal\" _=\"on closeModal add .closing then wait for animationend then remove me\">")

	fmt.Fprintln(w, "<div class=\"modal-underlay\" hx-trigger=\"click\" hx-target=\"#modal\" hx-swap=\"outerHTML\" hx-get=\"http://localhost:8888/nothing\"></div>")
	fmt.Fprintln(w, "<div class=\"modal-content\">")
	fmt.Fprintln(w, "<h2>Available Videos</h2>")
	fmt.Fprintln(w, "This is the modal content.")
	fmt.Fprintln(w, "<div id=\"sourceview\" hx-trigger=\"load, every 1s\" hx-get=\"http://localhost:8888/getrawsources\"></div>")

	fmt.Fprintln(w, "<div id=\"videoqueue\" hx-trigger=\"every 1s\" hx-get=\"http://localhost:8888/gettickets\"></div>")

	fmt.Fprintln(w, "<h2>Add video</h2>")
	fmt.Fprintln(w, "<form hx-get=\"http://localhost:8888/addyoutube\" hx-swap=none>")
	fmt.Fprintln(w, "<input placeholder=\"Paste youtube video or playlist link here\" id=\"addsource\" type=\"text\" name=\"url\"></input>")
	fmt.Fprintln(w, "<input id=\"addsource\" type=\"submit\"></input>")
	// <input id="search" type="search"
	// name="search" placeholder="Type to start searching"
	// hx-get="http://localhost:8888/query"
	// hx-trigger="keyup changed delay:5ms, search"
	// hx-target="#results"
	// hx-include="[class='source-checkbox']">

	fmt.Fprintln(w, "<br>")
	fmt.Fprintln(w, "<br>")
	fmt.Fprintln(w, "<button hx-trigger=\"click\" hx-target=\"#modal\" hx-swap=\"outerHTML\" hx-get=\"http://localhost:8888/nothing\">Close</button>")
	fmt.Fprintln(w, "</div>")
	fmt.Fprintln(w, "</div>")
	fmt.Fprintln(w, "</div>")
}

func main() {
	loadSources()

	mux := http.NewServeMux()
	mux.HandleFunc("/query", reqQuery)
	mux.HandleFunc("/addyoutube", reqAddYoutube)
	mux.HandleFunc("/getsources", reqGetSources)
	mux.HandleFunc("/getrawsources", reqGetRawSources)
	mux.HandleFunc("/gettickets", reqGetTickets)
	mux.HandleFunc("/sourcemodal", reqSourceModal)
	mux.HandleFunc("/nothing", func(w http.ResponseWriter, req *http.Request) {
		fmt.Println("")
	})
	mux.HandleFunc("/trolling", func(w http.ResponseWriter, req *http.Request) {
		fmt.Println("trolled!")
		fmt.Fprintln(w, "Yo wassup homie")
	})

	fmt.Println("Started server")

	handler := cors.AllowAll().Handler(mux)
	http.ListenAndServe(":8888", handler)
}
