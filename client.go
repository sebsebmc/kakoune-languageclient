package main

import (
	"bufio"
	"fmt"
	"github.com/sebsebmc/kakoune-languageclient/langsrvr"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type kakInstance struct {
	session, client, pipe string
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: kakoune-languageclient sessionId clientName")
		os.Exit(1)
	}
	session := os.Args[1]
	client := os.Args[2]
	tmpDir, _ := ioutil.TempDir("", "kakoune-languageclient")

	namedPipe := filepath.Join(tmpDir, session)
	syscall.Mkfifo(namedPipe, 0600)
	defer os.RemoveAll(namedPipe)

	instance := kakInstance{session, client, namedPipe}

	//hold the write end of the pipe so we dont get EOF
	fifo, _ := os.OpenFile(namedPipe, os.O_RDWR, os.ModeNamedPipe)

	//Create pipe first, then let Kakoune know about it
	instance.execCommand(fmt.Sprintf("decl str lsc_pipe %s", namedPipe))

	servers := make(map[string]*langsrvr.LangSrvr)

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
			//TODO: Try and make this a child of kak? Closing kak seems to close the spawned servers now...
			os.Exit(0)
		default:
			lang, cmd, args := tryParseCommand(string(line))
			instance.execCommand(fmt.Sprintf("echo -debug \"%s %s %s\"", lang, cmd, args))
			server, ok := servers[lang]
			if !ok {
				//Spawn a langserver for the language
				//TODO:
				server = langsrvr.NewLangSrvr("go-langserver")
				servers[lang] = server
				server.Initialize()
			}
			kakCmd, err := server.HandleKak(cmd, args)
			fmt.Println(kakCmd)
			if err == nil {
				instance.execCommand(kakCmd)
			}
		}
	}
}

func tryParseCommand(command string) (string, string, []string) {
	tokens := strings.Split(command, ":")
	if len(tokens) < 3 {
		return "", "", tokens
	}
	opts := strings.Split(tokens[2], ",")
	return tokens[0], tokens[1], opts
}

func (inst *kakInstance) execCommand(command string) {
	cmd := exec.Command("kak", "-p", inst.session)
	in, err := cmd.StdinPipe()
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
