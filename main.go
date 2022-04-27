package main

import(
	"fmt"
	fula "github.com/farhoud/go-fula/fula"
)

func main() {
	res := fula.StartLibp2p()
	if res {
		fmt.Printf("fula started")	
	} else {
		fmt.Printf("fula failed")
	}
    
}