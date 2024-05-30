package main

import (
	"fmt"
	"sync"
	"time"
)

type Store struct {
	m    sync.RWMutex
	data string
}

func writer1(s *Store) {
	for {
		time.Sleep(time.Millisecond * 1)
		if s.m.TryLock() {
			s.data += "1:"
			time.Sleep(time.Millisecond * 500) // simulate some work being done
			s.m.Unlock()
		}
	}
}

func writer2(s *Store) {
	for {
		time.Sleep(time.Millisecond * 1)
		if s.m.TryLock() {
			s.data += "2:"
			time.Sleep(time.Millisecond * 500) // simulate some work being done
			s.m.Unlock()
		}
	}
}

func reader(s *Store) {
	for {
		time.Sleep(time.Second)
		s.m.RLocker().Lock()
		if s.data != "" {
			fmt.Println(s.data)
		}
		s.m.RLocker().Unlock()
	}
}

func main() {
	s := &Store{
		m:    sync.RWMutex{},
		data: "",
	}

	var wg = &sync.WaitGroup{}

	wg.Add(3)

	go func() {
		defer wg.Done()

		writer1(s)
	}()

	go func() {
		defer wg.Done()

		writer2(s)
	}()

	go func() {
		defer wg.Done()

		reader(s)
	}()
	wg.Wait()

	fmt.Println(`Main done`)
}
