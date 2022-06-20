package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"

	fula "github.com/functionland/go-fula/mobile"
	filePL "github.com/functionland/go-fula/protocols/file"
	"github.com/golang/protobuf/proto"
)

func main() {

	fula, _ := fula.NewFula()
	fula.AddBox("/ip4/192.168.1.10/tcp/4002/p2p/12D3KooWGrkcHUBzAAuYhMRxBreCgofKKDhLgR84FbawknJZHwK1")
	fmt.Println("We are know connected")
	ref, err := fula.Send("/home/farhoud/test.txt")
	if err != nil {
		panic(err)
	}
	fmt.Println("cid", ref)
	buf, err := fula.ReceiveFileInfo(ref)
	meta := &filePL.Meta{}
	err = proto.Unmarshal(buf, meta)
	err = fula.ReceiveFile(ref, "/home/farhoud/"+ref+"-"+meta.Name)
	if err != nil {
		panic(err)
	}
	fmt.Println(meta.Name)
	query := `
	mutation addTodo($values:JSON){
	  create(input:{
		collection:"todo",
		values: $values
	  }){
		id
		text
		isComplete
	  }
	}
  `
	// query := `
	// query {
	//   read(input:{
	// 	collection:"todo",
	// 	filter:{text: {eq: "todo2"}}
	//   }){
	// 	id
	// 	text
	// 	isComplete
	//   }
	// }
	// `

	// [
	// 			{id: "1", text: "todo1", isComplete: false},
	// 			  {id: "2", text: "todo2", isComplete: false}
	// 		  ]
	//   var values map[string]interface{}
	//   values = append(values, map[string]interface{}{"id": "1", "text": "todo1", "isComplete": false})

	// values := map[string]interface{}{
	// 	"values": []interface{}{
	// 		map[string]interface{}{"id": "1", "text": "todo1", "isComplete": false},
	// 		map[string]interface{}{"id": "2", "text": "todo2", "isComplete": true}}}

	values := `{"values": [{"id": "1", "text": "todo1", "isComplete": false}, {"id": "2", "text": "todo2", "isComplete": true}]}`

	res, err := fula.GraphQL(query, values)
	if err != nil {
		panic(err)
	}
	fmt.Println(res)

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
