package main

import (
	"fmt"
	"net"
	"os"
)

func main() {

	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}
	consumeListener(l)

}

func consumeListener(l net.Listener) {
	for {
		connection, err := l.Accept()

		fmt.Println("Accepted new connection")

		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		fmt.Println("Spawning handler for new connection")
		go handleConnection(connection)
	}
}

func handleConnection(connection net.Conn) {
	for {
		buff := make([]byte, 1024)
		_, err := connection.Read(buff)
		if err != nil {
			fmt.Println("Connection closed")
			return
		}
		fmt.Println("Writing PONG response")
		connection.Write([]byte("+PONG\r\n"))
	}
}
