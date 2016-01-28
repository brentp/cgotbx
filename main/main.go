package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/brentp/cgotbx"
)

func main() {

	t, err := cgotbx.New(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	rdr, err := t.Get("1", 50000, 90000)
	if err != nil {
		log.Fatal(err)
	}
	str, err := ioutil.ReadAll(rdr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf(string(str))

}
