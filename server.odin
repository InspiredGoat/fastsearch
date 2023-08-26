package main

import "core:net"
import "core:os"
import "core:fmt"
import "core:thread"

// send and receive http requests
// 
// read transcription file


Source :: struct
{
    name: string,
    transcript: string,
     
}


run_cmd :: proc()
{
    pid, err := os.fork();

    if err != 0
    {
        fmt.eprintln("Failed to fork");
        os.exit(-1);
    }
    else
    {
        if (pid == 0) 
        {
            os.execvp("echo", {"hello world"})
        }
        else
        {
            return
        }
    }
}

main :: proc()
{
    // os.execvp("yt-dlp", {"-x", "--skip-download", "--audio-format", "wav", "https://www.youtube.com/watch?v=IaoeHb4duls" });
    run_cmd();
    fmt.println("OY this ran");
}
