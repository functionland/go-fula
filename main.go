package main

import (
	"fmt"
	"github.com/farhoud/go-fula/fula"
	"log"
	"os"
	"os/signal"
	"runtime"
	"time"
)

func main() {

	fula,_ := fula.NewFula("/home/farhoud")
	fula.Connect("/ip4/192.168.1.10/tcp/4002/p2p/12D3KooWDVgPHx45ZsnNPyeQooqY8VNesSR2KiX2mJwzEK5hpjpb")
	fmt.Println("We are know connected")
	cid,err := fula.Send("/home/farhoud/workspace/functionland/rngofula/libfula/test.txt")
	if err != nil {
		panic(err)
	}
	fmt.Println("cid", cid)
	time.Sleep(2 * time.Second)
	meta,err := fula.Receive(*cid)
	if err != nil {
		panic(err)
	}
	fmt.Println(meta)

	runtime.Goexit()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		select {
		case <-c:
			log.Printf("Close gracefully")
			signal.Stop(c)
			os.Exit(0)
		}
	}()
	fmt.Println("Exit")
	fmt.Println("R u running")

}
