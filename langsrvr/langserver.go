package langsrvr

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/powerman/rpc-codec/jsonrpc2"
	"io"
	"net/rpc"
	"os"
	"os/exec"
	"strconv"
)

type position struct {
	line      int
	character int
}

type textRange struct {
	start position
	end   position
}

type ioReadWriteCloser struct {
	io.ReadCloser
	io.WriteCloser
}

//From https://github.com/natefinch/pie
func (rw ioReadWriteCloser) Close() error {
	err := rw.ReadCloser.Close()
	if err := rw.WriteCloser.Close(); err != nil {
		return err
	}
	return err
}
func (rw ioReadWriteCloser) Write(buf []byte) (int, error) {
	fmt.Printf("--> %s\n", string(buf))
	contentLength := len(buf)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", contentLength)
	rw.WriteCloser.Write([]byte(header))
	n, err := rw.WriteCloser.Write(buf)
	return n, err
}

func (rw ioReadWriteCloser) Read(p []byte) (int, error) {
	headerReader := bufio.NewReader(rw.ReadCloser)
	headerReader.ReadLine()
	next, _ := headerReader.Peek(1)
	for !bytes.Equal(next, []byte("{")) {
		headerReader.ReadLine()
		next, _ = headerReader.Peek(1)
	}
	n, err := headerReader.Read(p)
	//fmt.Printf("<-- %s\n", string(p))
	return n, err
}

type LangSrvr struct {
	conn     *jsonrpc2.Client
	handlers map[string]func([]string)
}

func NewLangSrvr(command string) *LangSrvr {
	//Make a go-langserver to test
	server := exec.Command(command)
	fmt.Println("Starting langserver!")
	srvIn, err := server.StdinPipe()
	if err != nil {
		fmt.Printf("%s\n", err)
	}
	srvOut, err := server.StdoutPipe()
	if err != nil {
		fmt.Printf("%s\n", err)
	}
	serverConn := ioReadWriteCloser{srvOut, srvIn}
	err = server.Start()
	if err != nil {
		fmt.Printf("%s\n", err)
	}
	var lspRPC LangSrvr
	lspRPC.handlers = make(map[string]func([]string))
	lspRPC.conn = jsonrpc2.NewClient(serverConn)
	return &lspRPC
}

func (ls *LangSrvr) Initialize() {
	wd, _ := os.Getwd()
	fileUri := "file://" + wd
	params := map[string]interface{}{"processId": os.Getpid(), "rootUri": fileUri, "rootPath": wd,
		"capabilities": make(map[string]interface{})}
	//TODO: use capabilities to assign handlers
	ls.handlers["textDocument/hover"] = ls.tdHover
	ls.handlers["textDocument/signatureHelp"] = ls.tdSigHelp
	ls.execCommandSync("initialize", params)
	ls.notify("initialized", nil)
}

func (ls *LangSrvr) Handle(cmd string, args []string) {
	handler, ok := ls.handlers[cmd]
	if ok == false {
		return
	}
	handler(args)
}

func (ls *LangSrvr) Shutdown() {
	ls.execCommandSync("shutdown", map[string]interface{}{})
}

/*
* buffile, line, charachter
* textDocument/hover
* params:{textDocument:URI, position:{line:,character:}}
 */
func (ls *LangSrvr) tdHover(params []string) {
    fmt.Printf("hover: %s\n", params)
	uri := "file://" + params[0]
	line, _ := strconv.Atoi(params[1])
	character, _ := strconv.Atoi(params[2])
	paramMap := map[string]interface{}{
		"textDocument": map[string]string{"uri": uri},
		"position": map[string]interface{}{
			"line":      line - 1,
			"character": character - 1,
		},
	}
	ls.execCommandSync("textDocument/hover", paramMap)
}

/*
* textDocument/signatureHelp
* params:{textDocument:URI, position:{line:,character:}}
 */
func (ls *LangSrvr) tdSigHelp(params []string) {
    fmt.Printf("sigHelp: %s\n", params)
	uri := "file://" + params[0]
	line, _ := strconv.Atoi(params[1])
	character, _ := strconv.Atoi(params[2])
	paramMap := map[string]interface{}{
		"textDocument": map[string]string{"uri": uri},
		"position": map[string]interface{}{
			"line":      line - 1,
			"character": character - 1,
		},
	}
	ls.execCommandSync("textDocument/signatureHelp", paramMap)
}

func (ls *LangSrvr) execCommandSync(command string, params map[string]interface{}) map[string]interface{} {
	var reply map[string]interface{}
	err := ls.conn.Call(command, params, &reply)
	if err == rpc.ErrShutdown || err == io.ErrUnexpectedEOF {
		fmt.Printf("Err1(): %q\n", err)
	} else if err != nil {
		rpcerr := jsonrpc2.ServerError(err)
		fmt.Printf("Err1(): code=%d msg=%q data=%v\n", rpcerr.Code, rpcerr.Message, rpcerr.Data)
	}
	fmt.Println(reply)
	return reply
}

func (ls *LangSrvr) notify(method string, args interface{}) {
	ls.conn.Notify(method, args)
}
