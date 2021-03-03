package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

func main() {

	log.SetFlags(log.LstdFlags | log.LUTC)

	mux := &http.ServeMux{}

	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/events", eventsHandler)

	s := http.Server{
		Addr:    "localhost:8000",
		Handler: mux,
	}

	// disable http keep-alive
	// s.SetKeepAlivesEnabled(false)

	log.Println("Server started!")
	// log.Fatal(s.ListenAndServe())
	log.Fatal(s.ListenAndServeTLS("cert.pem", "key.pem"))
}

func eventsHandler(rw http.ResponseWriter, r *http.Request) {
	// preparing the waitgroup and channels
	wg := &sync.WaitGroup{}
	// add timeout
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	messageChannel := make(chan string)
	// dont forget to close the channel
	defer close(messageChannel)

	// setting all of the necesarry header for SSE
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("X-Accel-Buffering:", "no")

	// spawn 4 goroutine to generate message
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go worker(i+1, r, messageChannel, wg)
	}

	// spawn another goroutine to consume the messageChannel and write it to rw (http.ResponseWriter)
	wg.Add(1)
	go mesageSender(rw, messageChannel)

	// .... consume the request body, parse form value, handle upload file(s) etc.`

	// i'm not put this in defer, to make it more clear. using defer is absolutely easier.
	// closing the body
	_ = r.Body.Close()
	// and waiting for all of the goroutine to finish
	wg.Wait()
}

func worker(x int, r *http.Request, messageChannel chan string, wg *sync.WaitGroup) {
	defer wg.Done()
the_loop:
	for {
		log.Println("worker number", x, "doing some job...")
		// produce messages and send it to the channel ...
		messageChannel <- fmt.Sprintf("message from worker %d %s", x, time.Now().UTC().String())
		time.Sleep(2 * time.Second)
		select {
		case <-r.Context().Done():
			break the_loop
		default:
		}
	}
	log.Println("worker number", x, "return", r.Context().Err())
}

func mesageSender(rw http.ResponseWriter, messageChannel chan string) {
	flusher, ok := rw.(http.Flusher)
	if !ok {
		log.Println("Client does not suppor SSE")
		return
	}

	for each := range messageChannel {
		// Send the message to the client (write it to http.ResponseWriter)
		fmt.Fprintf(rw, "data: %s\n\n", each)
		// flush the data to the client
		flusher.Flush()
	}
	log.Println("mesageSender has returned.")
}
