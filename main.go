package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	librlcom "github.com/RIscRIpt/rrl/go-librlcom"
)

const (
	listenProtocol string = "unix"
)

var (
	flagListenPath = flag.String("addr", "/var/run/svcsymres.sock", "listen path")
	flagLibsPath   = flag.String("libs", "./libs", "path to directory with .lib files")
)

var SymRes *SymbolResolver

func main() {
	var err error

	flag.Parse()

	SymRes, err = NewSymbolResolver(*flagLibsPath)
	if err != nil {
		log.Fatalln(err)
	}

	listener, err := net.Listen(listenProtocol, *flagListenPath)
	if err != nil {
		log.Fatalln(err)
	}

	exitSignal := make(chan os.Signal)
	signal.Notify(exitSignal,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	go handleClients(listener)

	s := <-exitSignal
	log.Printf("received signal `%s`, exitting...\n", s.String())

	listener.Close()
}

func handleClients(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error occured on `listener.Accept`\nError: %v\n", err)
		}
		go handleClient(conn)
	}
}

func handleClient(conn net.Conn) {
	log.Printf("Client connected from %+v\n", conn.RemoteAddr())

	c := librlcom.NewCourier(conn)

loop:
	for {
		msg, err := c.Receive()
		switch err {
		case nil:
		case librlcom.ErrUnknownMessage:
			header := msg.(*librlcom.Header)
			log.Println(err, header)
			break loop
		case io.EOF:
			break loop
		default:
			log.Println(err)
			break loop
		}

		switch m := msg.(type) {
		case *librlcom.GetSymbolLibrary:
			library, err := SymRes.Resolve(m.String.String())
			if err != nil {
				log.Println(err)
			}
			err = c.Send(&librlcom.ResolvedSymbolLibrary{
				String: librlcom.String(library),
			})
			if err != nil {
				log.Println(err)
			}
		default:
			log.Println(librlcom.ErrUnknownMessage, m)
		}
	}

	log.Printf("Client disconnected from %+v\n", conn.RemoteAddr())

	conn.Close()
}
