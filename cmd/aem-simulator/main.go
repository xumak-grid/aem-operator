package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	fmt.Println("Initializing AEM Simulator")
	runmode := os.Getenv("CQ_RUNMODE")
	p := ":4502"
	if strings.Contains(runmode, "publish") {
		p = ":4503"
	}
	mux := http.NewServeMux()
	status := http.StatusNotFound
	<-time.After(time.Second * 100)
	m := &sync.Mutex{}
	go func() {
		<-time.After(time.Second * 15)
		m.Lock()
		status = http.StatusServiceUnavailable
		m.Unlock()
		<-time.After(time.Second * 15)
		m.Lock()
		status = http.StatusOK
		m.Unlock()
	}()
	mux.HandleFunc("/system/health", func(w http.ResponseWriter, r *http.Request) {
		m.Lock()
		fmt.Println("Writing status: ", status)
		w.WriteHeader(status)
		m.Unlock()
		fmt.Fprintln(w, "AEM Simulator Health Check")
	})
	fmt.Println("Server started to listen at: ", p)
	http.ListenAndServe(p, mux)
}
