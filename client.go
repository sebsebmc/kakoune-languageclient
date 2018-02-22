package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type kakInstance struct {
	session, client, pipe string
}

type kakBuffer struct {
	file     string   // Don't actually open the file, we don't need to read it, the lsp does
	tmpFile  *os.File // Where we can write the buffer for syncing. Allows reuseability
	language string
	lastEdit int
	lastSync int // So we know if we synced (i.e when this is 0)
}

type lspCommand struct {
	command string
	args    []string
}

var servers map[string]*LangSrvr
var buffers map[string]*kakBuffer

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: kakoune-languageclient sessionId clientName")
		os.Exit(1)
	}
	session := os.Args[1]
	client := os.Args[2]
	tmpDir, _ := ioutil.TempDir("", "lsc")

	namedPipe := filepath.Join(tmpDir, session)
	syscall.Mkfifo(namedPipe, 0600)
	defer os.RemoveAll(namedPipe)

	instance := kakInstance{session, client, namedPipe}

	//hold the write end of the pipe so we dont get EOF
	fifo, _ := os.OpenFile(namedPipe, os.O_RDWR, os.ModeNamedPipe)

	//Create pipe first, then let Kakoune know about it
	instance.execCommand(fmt.Sprintf("decl str lsc_pipe %s", namedPipe))

	servers = make(map[string]*LangSrvr)
	buffers = make(map[string]*kakBuffer)

	reader := bufio.NewReader(fifo)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		switch string(line) {
		case "Ping":
			instance.execCommand("echo -debug Pong\n")
		case "KakEnd":
			//TODO: shutdown servers
			//TODO: remove temp files
			//TODO: Try and make this a child of kak? Closing kak seems to close the spawned servers now...
			os.Exit(0)
		default:
			buf, cmd := tryParseCommand(string(line))
			if buf != nil && cmd != nil {
				fmt.Printf("%v\n%v\n", buf, cmd)
				instance.execCommand(fmt.Sprintf("echo -debug \"%s %s\"", cmd.command, cmd.args))
				handleCommand(&instance, buf, cmd)
			}
		}
	}
}

func handleCommand(instance *kakInstance, buf *kakBuffer, cmd *lspCommand) {
	server := getServer(buf.language)
	if buf.lastEdit > buf.lastSync {
		if buf.lastSync == 0 {
			tmpFile, err := ioutil.TempFile("", "lsc-tempBuf")
			if err != nil {
				fmt.Println("Cannot create tmp file to sync")
				return
			}
			buf.tmpFile = tmpFile
		}
		instance.execCommand(fmt.Sprintf("eval write -no-hooks %s", buf.tmpFile.Name()))
		server.HandleKak(buf, &lspCommand{command: "textDocument/sync"})
	}
	kakCmd, err := server.HandleKak(buf, cmd)
	fmt.Println(kakCmd) //debug printing
	if err == nil {
		instance.execCommand(kakCmd)
	}
}

// Gets a server instance for a given filetype, launching and
// initializing the connection as necessary
func getServer(lang string) *LangSrvr {
	server, ok := servers[lang]
	if !ok {
		//Spawn a langserver for the language
		//TODO: read mapping of filetype to command from a config
		server = NewLangSrvr("go-langserver")
		servers[lang] = server
		server.Initialize()
	}
	return server
}

// string from kak formatted like
// filetype:filename:editTimestamp:command:args1,args2,...
func tryParseCommand(command string) (*kakBuffer, *lspCommand) {
	tokens := strings.Split(command, ":")
	if len(tokens) < 5 {
		return nil, nil
	}

	buf, ok := buffers[tokens[1]]
	opts := strings.Split(tokens[4], ",")
	cmd := &lspCommand{tokens[3], opts}
	if !ok {
		// Set dirty for new buffers since we don't know if they're pristine
		buf = &kakBuffer{language: tokens[0], file: tokens[1]}
		//TODO: Do I need a document/didOpen here before I can request a didChange?
		// If so then I need to track if didOpen was called (Maybe don't add to buffers?
		buffers[tokens[1]] = buf
	}

	timestamp, err := strconv.Atoi(tokens[2])
	if err != nil {
		timestamp = 0
	}
	buf.lastEdit = timestamp
	
	return buf, cmd
}

func (inst *kakInstance) execCommand(command string) {
	cmd := exec.Command("kak", "-p", inst.session)
	in, err := cmd.StdinPipe()
	// Needs investigating: can you hold just one of these pipes and keep sending
	// commands instead of spawning a new process evry time? Could drop latencies
	if err != nil {
		fmt.Println("Failed to get the stdin pipe!")
	}
	//cmd.Stdout = os.Stdout
	//There is no Stdout for a -p
	cmd.Start()
	in.Write([]byte(fmt.Sprintf("eval -client %s \"%s\"", inst.client, command)))
	in.Close()
	cmd.Wait()
}
