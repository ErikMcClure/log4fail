package main

import (
  "github.com/miekg/dns"
  "sync"
  "sync/atomic"
  "time"
  "container/heap"
  "strconv"
  "strings"
  "fmt"
)

type Timeout struct {
  t int64
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

var hits = make(map[uint64]uint64)
var timeouts = make(MinHeap, 1000)

var counter uint32 = 0
var mainlock = &sync.RWMutex{}
var heaplock = &sync.RWMutex{}

func ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
  q := r.Question[0]
  fmt.Println(q.Name)
  id, err := strconv.ParseUint(strings.Split(q.Name, ".")[0], 16, 64)
  
  if err != nil {
    mainlock.Lock()
    v, ok := hits[id]
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

func AddSubzone() uint64 {
  // Create simplified snowflake
  ms := time.Now().UnixMilli()
  var id uint64 = uint64(ms) & 0xFFFFF
  id = id | (uint64(MachineID) << 40)
  id = id | (uint64(atomic.AddUint32(&counter, 1) & 0xFF) << 48)
  
  mainlock.Lock()
  hits[id] = 0
  mainlock.Unlock()
  heaplock.Lock()
  heap.Push(&timeouts, Timeout{t: ms, snowflake: id});
  heaplock.Unlock()
  
  return id
}

const MAX_TIMEOUT int64 = 50000000

func Cleanup() {
  mainlock.Lock()
  defer mainlock.Unlock()
  heaplock.Lock()
  defer heaplock.Unlock()
  
  ms := time.Now().UnixMilli()
  
  for timeouts[0].t + MAX_TIMEOUT < ms {
    delete(hits, timeouts[0].snowflake)
    heap.Pop(&timeouts)
  }
}

func main() {
  heap.Init(&timeouts)
  
  server := &dns.Server{Addr: "localhost:53", Net: "udp"}
  go server.ListenAndServe()
  dns.HandleFunc(".", ServeDNS)
  
  for {}
}
