package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	fmt.Println("Frontend running on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}