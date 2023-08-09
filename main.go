package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type p2st struct {
	pall map[int64]*people
	ch   chan [][]byte
	sync.RWMutex
}

type people struct {
	p int64
	r *net.TCPConn
}

var unionId = new(int64)
var unionId2 = new(uint32)
var p = new(p2st)
var da = new(Frames)

func main() {
	var tadd, _ = net.ResolveTCPAddr("tcp", ":61137")
	d, e := net.ListenTCP("tcp", tadd)
	if e != nil {
		fmt.Println(e)
		return
	}
	p.init()
	go runFrames()
	for {
		ac, e := d.AcceptTCP()
		if e != nil {
			fmt.Println(e)
			return
		}
		onep := p.add(ac)
		go func() {
			for {
				var bbb = make([]byte, 1024)
				l, e := ac.Read(bbb)
				if e != nil {
					p.del(onep.p)
					return
				}
				da.ch <- bbb[:l]
			}
		}()
	}
}

func (p *p2st) init() {
	p.pall = make(map[int64]*people)

	p.ch = make(chan [][]byte, 1024)
	go func() {
		var data [][]byte
		for {
			select {
			case data = <-p.ch:
				waitdata := make([]byte, 0, 1024)
				if len(data) > 1 {
					for k, v := range data {
						if k == 0 {
							waitdata = data[0]
							continue
						}
						waitdata = append(waitdata, []byte("\r\n")...)
						waitdata = append(waitdata, v...)
					}
				} else if len(data) == 1 {
					waitdata = data[0]
				}
				var rs []*people //防止卡顿影响锁
				p.RWMutex.RLock()
				for _, v := range p.pall {
					rs = append(rs, v)
				}
				p.RWMutex.RUnlock()
				for _, v := range rs {
					_, e := v.r.Write(waitdata)
					if e != nil {
						p.del(v.p)
						continue
					}
				}
			}
		}
	}()
}

func (p *p2st) del(id int64) {
	fmt.Println("删除连接:", id)
	p.RWMutex.Lock()
	d := p.pall[id]
	p.RWMutex.Unlock()
	if d != nil {
		d.r.Close()
	}

	p.Lock()
	delete(p.pall, id)
	p.Unlock()
}
func (p *p2st) add(r *net.TCPConn) *people {
	id := atomic.AddInt64(unionId, 1)
	onep := &people{id, r}
	p.RWMutex.Lock()
	p.pall[id] = onep
	p.RWMutex.Unlock()
	fmt.Println("建立连接:", id)
	return onep
}

type Frames struct {
	t  time.Time
	ch chan []byte
}

func runFrames() {
	fmt.Println("q启动帧同步")
	da.ch = make(chan []byte, 1024)
	var t = time.NewTicker(time.Millisecond * 33)
	var bo bool
	var data = make([][]byte, 0, 1024)
	for {
		bo = false
		id := atomic.AddUint32(unionId2, 1)
		var wd []byte
		wd = binary.BigEndian.AppendUint32(wd, id)
		data = [][]byte{wd}
		for {
			select {
			case d := <-da.ch:
				data = append(data, d)
			case <-t.C:
				bo = true
			default:
			}
			if bo {
				break
			}
		}

		p.handle(data)
	}
}

func (p *p2st) handle(data [][]byte) {
	p.ch <- data
}
