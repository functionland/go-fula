package main

import (
	"fmt"
	"github.com/farhoud/go-fula/fula"
	"log"
	"os"
	"os/signal"
	"runtime"
	// "time"
)

func main() {

	fula,_ := fula.NewFula("/home/mehdi")
	fula.Connect("/ip4/192.168.246.234/tcp/4002/p2p/12D3KooWRDBvNPv7zPrXoo7KwhpMxgS3Qga4tKKjtbexQnGSvdni")
	fmt.Println("We are know connected")
	// cid,err := fula.Send("/home/mehdi/test.txt")
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println("cid", cid)
	// time.Sleep(2 * time.Second)
	// meta,err := fula.Receive(cid)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println(meta)
	query := `
	mutation addTodo($values:JSON){
	  create(input:{
		collection:"todo",
		values: [
			{id: "1", text: "todo1", isComplete: false},
			  {id: "2", text: "todo2", isComplete: false}
		  ]
	  }){
		id
		text
		isComplete
	  }
	}
  `
	  values := map[string]interface{}{}
	  
	res, err := fula.GraphQL(query, values)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(res))

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
