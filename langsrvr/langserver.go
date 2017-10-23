package langsrvr

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/powerman/rpc-codec/jsonrpc2"
	"io"
	"net/rpc"
	"os"
	"os/exec"
	"strconv"
)

type Position struct {
	Line      int
	Character int
}

type textRange struct {
	Start Position
	End   Position
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
	fmt.Printf("<-- %s\n", string(p))
	return n, err
}

type LangSrvr struct {
	conn     *jsonrpc2.Client
	handlers map[string]func([]string) string
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
	lspRPC.handlers = make(map[string](func([]string) string))
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
	ls.execCommandSync("initialize", params, nil)
	ls.notify("initialized", nil)
}

func (ls *LangSrvr) Handle(cmd string, args []string) (string, error) {
	handler, ok := ls.handlers[cmd]
	if ok == false {
		return "", errors.New("Command does not exist")
	}
	return handler(args), nil
}

func (ls *LangSrvr) Shutdown() {
	ls.execCommandSync("shutdown", map[string]interface{}{}, nil)
}

//=======================================================

/*
* buffile, line, charachter
* textDocument/hover
* params:{textDocument:URI, position:{line:,character:}}
 */
func (ls *LangSrvr) tdHover(params []string) string {
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
	type MarkedString struct {
		Language string `json:"language,omitempty"`
		Value    string `json:"value,omitempty"`
		Simple   string `json:"simple,omitempty"`
	}
	reply := struct {
		Docs   []MarkedString `json:"contents"`
		Ranges textRange      `json:"range,omitempty"`
	}{}
	ls.execCommandSync("textDocument/hover", paramMap, &reply)
	fmt.Println(reply)
	return fmt.Sprintf("info -placement above -anchor %s.%s '%s'", params[1], params[2], reply.Docs[0])
}

/*
* textDocument/signatureHelp
* params:{textDocument:URI, position:{line:,character:}}
 */
func (ls *LangSrvr) tdSigHelp(params []string) string {
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
	type sigInfo struct {
		Label  string                   `json:"label"`
		Docs   string                   `json:"documentation,omitempty"`
		Params []map[string]interface{} `json:"parameters,omitempty"`
	}
	reply := struct {
		Signatures []sigInfo `json:"signatures"`
		AParam     int       `json:"activeParameter,omitempty"`
		ASig       int       `json:"activeSignature,omitempty"`
	}{}
	err := ls.execCommandSync("textDocument/signatureHelp", paramMap, &reply)
	if err != nil {
		return "echo 'Command failed'"
	}
	fmt.Println(reply)
	return fmt.Sprintf("info -placement above -anchor %s.%s '%s\n%s'", params[1], params[2], reply.Signatures[reply.ASig].Label, reply.Signatures[reply.ASig].Docs)
}

func (ls *LangSrvr) execCommandSync(command string, params map[string]interface{}, reply interface{}) error {
	err := ls.conn.Call(command, params, reply)
	if err == rpc.ErrShutdown || err == io.ErrUnexpectedEOF {
		fmt.Printf("Err1(): %q\n", err)
		return errors.New("RPC Error")
	} else if err != nil {
		rpcerr := jsonrpc2.ServerError(err)
		fmt.Printf("Err1(): code=%d msg=%q data=%v\n", rpcerr.Code, rpcerr.Message, rpcerr.Data)
		return errors.New("JSONRPC Error")
	}
	return nil
}

func (ls *LangSrvr) notify(method string, args interface{}) {
	ls.conn.Notify(method, args)
}
