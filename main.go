package main

import (
	"container/heap"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/miekg/dns"
)

type Timeout struct {
	t         int64
	snowflake uint64
}

type MinHeap []Timeout

func (h MinHeap) Len() int           { return len(h) }
func (h MinHeap) Less(i, j int) bool { return h[i].t < h[j].t }
func (h MinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MinHeap) Push(x interface{}) {
	*h = append(*h, x.(Timeout))
}

func (h *MinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

var hits = make(map[uint64]int64)
var mapping = make(map[uint64]uint64)
var timeouts = make(MinHeap, 1000)

var counter uint32 = 0
var mainlock = &sync.RWMutex{}
var heaplock = &sync.RWMutex{}

func ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	q := r.Question[0]
	fmt.Println(q)
	s := strings.Split(q.Name, ".")[0]
	id, err := strconv.ParseUint(s, 16, 64)
	fmt.Println(fmt.Sprintf("%s (%s): %d %x", q.Name, s, id, id), err)

	if err == nil {
		mainlock.Lock()
		v, ok := hits[id]
		fmt.Println(v, ok)
		if ok {
			hits[id] = v + 1
		}
		mainlock.Unlock()
	}

	m := new(dns.Msg)
	m.Authoritative = true
	m.SetReply(r)
	m.Rcode = dns.RcodeNameError
	w.WriteMsg(m)
}

const MachineID uint8 = 1

func UnixMilli(t time.Time) int64 {
	return t.Unix()*1e3 + int64(t.Nanosecond())/1e6
}

func AddSubzone() (uint64, uint64) {
	// Create simplified snowflake
	ms := UnixMilli(time.Now())
	var id uint64 = uint64(ms) & 0xFFFFF
	id = id | (uint64(MachineID) << 40)
	id = id | (uint64(atomic.AddUint32(&counter, 1)&0xFF) << 48)

	id_probe := id | (1 << 47)

	mainlock.Lock()
	hits[id] = 0
	hits[id_probe] = 0
	mapping[id] = id_probe
	mainlock.Unlock()
	heaplock.Lock()
	heap.Push(&timeouts, Timeout{t: ms, snowflake: id})
	heap.Push(&timeouts, Timeout{t: ms, snowflake: id_probe})
	heaplock.Unlock()

	return id, id_probe
}

const MAX_TIMEOUT int64 = 50000000

func Cleanup() {
	mainlock.Lock()
	defer mainlock.Unlock()
	heaplock.Lock()
	defer heaplock.Unlock()

	ms := UnixMilli(time.Now())

	for timeouts[0].t+MAX_TIMEOUT < ms {
		delete(hits, timeouts[0].snowflake)
		heap.Pop(&timeouts)
	}
}

func createHandler(w http.ResponseWriter, r *http.Request) {
	id1, id2 := AddSubzone()
	fmt.Fprintf(w, "%x,%x", id1, id2)
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(mux.Vars(r)["id"], 16, 64)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		var sub int64 = 0
		mainlock.RLock()
		count, ok := hits[id]
		if probe, exists := mapping[id]; exists {
			sub, _ = hits[probe]
		}
		mainlock.RUnlock()

		if !ok {
			w.WriteHeader(http.StatusNotFound)
		} else {

			fmt.Fprintf(w, "%d", count-sub)
		}
	}
}

func main() {
	heap.Init(&timeouts)

	dns.HandleFunc(".", ServeDNS)
	server := &dns.Server{Addr: ":53", Net: "udp"}
	go server.ListenAndServe()
	tcpserver := &dns.Server{Addr: ":53", Net: "tcp"}
	go tcpserver.ListenAndServe()

	router := mux.NewRouter().StrictSlash(false)
	router.HandleFunc("/id", createHandler).Methods("POST")
	router.HandleFunc("/id/{id}", getHandler).Methods("GET")
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./static")))

	srv := &http.Server{
		Addr:         "0.0.0.0:8080",
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      router, // Pass our instance of gorilla/mux in.
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("Shutting down...")
	os.Exit(0)
}
