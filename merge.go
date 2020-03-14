package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"regexp"
)

const COLOR_FORMAT string = "\033[3%dm%s\033[0m"
const NUM_COLORS = 6
var TRIM_COLORS_PATTERN *regexp.Regexp

var messageBuffer = make(chan message)
var processWait sync.WaitGroup

type message struct {
	content string
	sender  int
}

func (output message) String() string {
	return fmt.Sprintf(COLOR_FORMAT, (output.sender%NUM_COLORS)+1, output.content)
}

func readPipe(pipe io.ReadCloser, mergedOutput chan string, wait *sync.WaitGroup) {
	defer wait.Done()
	output := bufio.NewScanner(pipe)
	for output.Scan() {
		mergedOutput <- TRIM_COLORS_PATTERN.ReplaceAllString(output.Text(), "")
	}
}

// Sends both stdout and stderror of command to mergedOutput
func mergeOutErr(command *exec.Cmd, mergedOutput chan string) {
	stdout, err := command.StdoutPipe()
	if err != nil {
		panic(fmt.Sprintf("Could not connect to stdout of process '%s'. Error: %s", command.Path, err))
	}
	stderror, err := command.StderrPipe()
	if err != nil {
		panic(fmt.Sprintf("Could not connect to stderr of process '%s'. Error: %s", command.Path, err))
	}
	if err := command.Start(); err != nil {
		panic(fmt.Sprintf("Could not start process '%s'. Error: %s", command.Path, err))
	} else {
		mergedOutput <- fmt.Sprintf("Started '%s' (%d)", command.Path, command.Process.Pid)
	}
	var both sync.WaitGroup
	both.Add(2)
	go readPipe(stdout, mergedOutput, &both)
	go readPipe(stderror, mergedOutput, &both)
	both.Wait()
	close(mergedOutput)
}

func listenTo(command *exec.Cmd, id int) {
	defer processWait.Done()
	mergedOutput := make(chan string)
	go mergeOutErr(command, mergedOutput)
	for line := range mergedOutput {
		messageBuffer <- message{line, id}
	}
	if err := command.Wait(); err != nil {
		messageBuffer <- message{fmt.Sprintf("Process '%s' (%d) exited with %s", command.Path, command.Process.Pid, err), id}
	} else {
		messageBuffer <- message{fmt.Sprintf("Process '%s' (%d) exited successfully", command.Path, command.Process.Pid), id}
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: must supply at least two processes to run")
		os.Exit(1)
	}
	TRIM_COLORS_PATTERN = regexp.MustCompile("\033\\[[^m]*m")
	processWait.Add(len(os.Args) - 1)
	for i, cmd := range os.Args[1:] {
		fields := strings.Fields(cmd)
		go listenTo(exec.Command(fields[0], fields[1:]...), i)
	}
	go (func() {
		processWait.Wait()
		close(messageBuffer)
	})()
	for output := range messageBuffer {
		fmt.Println(output)
	}
}
