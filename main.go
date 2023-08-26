package main

import (
	"fmt"
	"os/exec"
)

type Lennox struct {
}

type Source struct {
	name      string
	subtitles string
}

func main() {

	exec.Command("yt-dlp", "-x", "--audio-format", "wav", "https://www.youtube.com/watch?v=IaoeHb4duls")

	fmt.Println("hello world")
}
