package util

import (
	"fmt"
	"sync"
	"time"
)

type IdGen struct {
	Prefix string
	Id     int64
	*sync.Mutex
}

func NewIdGen(prefix string) *IdGen {
	date := time.Now().Format("2006-01-02")
	return &IdGen{
		Mutex:  new(sync.Mutex),
		Prefix: fmt.Sprintf("%s.%s", prefix, date),
		Id:     0,
	}
}

func (gen *IdGen) Get() string {
	gen.Lock()
	defer gen.Unlock()
	gen.Id++
	return fmt.Sprintf("%s.%d", gen.Prefix, gen.Id)
}
