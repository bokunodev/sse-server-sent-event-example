package main

import (
	"bufio"
	"container/list"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type subscriber struct {
	net.Conn
	bufio.ReadWriter
}

var subscribers = list.New()
var subscribersMutex = sync.Mutex{}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	go notifier()

	http.Handle("/",
		http.FileServer(http.Dir(".")))

	http.HandleFunc("/subscribe", sub)

	log.Printf("Open http://localhost:8000/ on your browser...")
	log.Println("Send SIGHUP to pid:", os.Getpid(), "to broadcast notification to user(s)")

	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func notifier() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)

	defer func() {
		signal.Stop(sigCh)
		close(sigCh)
	}()

	// start main loop
	for range sigCh {
		subscribersMutex.Lock()
		if subscribers.Len() > 0 {
			for elem := subscribers.Front(); elem != nil; elem = elem.Next() {
				client, _ := elem.Value.(*subscriber)
				_, err := client.ReadWriter.WriteString("10\r\ndata: hello :)\n\n\r\n")
				if err != nil {
					log.Println("client disconnected")
					client.Conn.Close()
					subscribers.Remove(elem)
					continue
				}
				err = client.ReadWriter.Flush()
				if err != nil {
					log.Println("client disconnected")
					client.Conn.Close()
					subscribers.Remove(elem)
					continue
				}
			}
		}

		subscribersMutex.Unlock()
	}
	// end main loop
}

func sub(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Transfer-Encoding", "chunked")
	rw.Header().Set("Cache-Control", "no-cache")

	f, ok := rw.(http.Flusher)
	if !ok {
		log.Println("client doesn't support streaming")
		rw.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	f.Flush()

	h, ok := rw.(http.Hijacker)
	if !ok {
		log.Println("server doesn't support hijacking")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := h.Hijack()
	if err != nil {
		log.Println("server failed to hijack the connection")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	subscribersMutex.Lock()
	defer subscribersMutex.Unlock()

	subscribers.PushFront(&subscriber{Conn: conn, ReadWriter: *bufrw})
}
