package main

import (
	"fmt"
	"sync"
	"time"
)

func producer() chan int {
	ch := make(chan int, 10)

	go func() {
		defer close(ch)

		i := 0
		for {
			ch <- i
			i++
			time.Sleep(time.Millisecond * 500) // simulate some work being done
		}
	}()

	return ch
}

func fanOut(in chan int) (chan int, chan int) {
	outA := make(chan int)
	outB := make(chan int)

	go func() {
		defer func() {
			close(outA)
			close(outB)
		}()

		for d := range in {
			select {
			case outA <- d:
			case outB <- d:
			}
		}
	}()

	return outA, outB
}

func main() {
	ch := producer()
	a, b := fanOut(ch)

	var wg = sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for v := range a {
			println("a:", v)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for v := range b {
			println("b:", v)
		}
	}()

	wg.Wait()

	fmt.Println(`Main done`)
}
