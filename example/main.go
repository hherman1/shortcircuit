package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/hherman1/shortcircuit"
	"golang.org/x/net/html"
	"net/http"
	"os"
	"strconv"
	"strings"
)

//go:embed sample.html
var sample string

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}


func run() error {
	http.Handle("/ws", http.HandlerFunc(handleWs))
	return http.ListenAndServe(":8080", nil)
}

func handleWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("upgrade", err)
		return
	}
	go func() {
		defer conn.Close()
		err := client(conn)
		if err != nil {
			fmt.Println("talking to websocket:", err)
			return
		}
	}()
}

// Manages an interaction with a new websocket client. Does not close the socket.
func client(conn *websocket.Conn) error {

	// assume we're starting with the sampe sample
	hn, err := html.Parse(strings.NewReader(sample))
	if err != nil {
		return fmt.Errorf("parse sample html: %w", err)
	}
	var cl shortcircuit.Changelog
	n := shortcircuit.Node{
		N:  hn,
		Cl: &cl,
	}
	counter := 0
	//	flicker, err := parse(`<div>
	//<h1> This flickering div comes from the backend!!! </h1>
	//<p> this too </p>
	//</div>`)
	//	if err != nil {
	//		return fmt.Errorf("parse flicker html: %w", err)
	//	}
	for {
		var event struct {Type string; Message string}
		err := conn.ReadJSON(&event)
		if err != nil {
			return fmt.Errorf("read event: %w", err)
		}
		body := n.Body()
		cnode := body.ById("counter")
		counter++
		cnode.Rm(0)
		newCnode, err := shortcircuit.Parse(strconv.Itoa(counter))
		if err != nil {
			return fmt.Errorf("parse counter: %w", err)
		}
		cnode.Insert(newCnode, 0)
		// flush
		err = conn.WriteJSON(cl.Buffer)
		if err != nil {
			return fmt.Errorf("flush insert: %w", err)
		}
		cl.Buffer = cl.Buffer[:0]
		//time.Sleep(1 * time.Second)
		//body.rm(0)
		//// flush again
		//err = conn.WriteJSON(cl.buffer)
		//if err != nil {
		//	return fmt.Errorf("flush rm: %w", err)
		//}
		//cl.buffer = cl.buffer[:0]
	}
}

func show(n *html.Node) {
	var s bytes.Buffer
	err := html.Render(&s, n)
	if err != nil {
		panic(fmt.Errorf("render path result: %w", err))
	}
	fmt.Println(s.String())
}
