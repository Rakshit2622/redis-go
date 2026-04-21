package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

type RespValue struct {
	typ     byte   // '+', '-', ':', '$', '*'
	str     string // simple string, error, bulkstring
	integer int
	array   []RespValue
}

type CommandHandler func(args []RespValue) string

var commands = map[string]CommandHandler{
	"PING": handlePing,
	"ECHO": handleEcho,
}

func main() {

	l, err := net.Listen("tcp", "0.0.0.0:6379")
	if err != nil {
		fmt.Println("Failed to bind to port 6379")
		os.Exit(1)
	}
	consumeListener(l)
}

func consumeListener(l net.Listener) {
	defer l.Close()
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
	defer connection.Close()
	for {
		buff := make([]byte, 1024)
		_, err := connection.Read(buff)
		if err != nil {
			fmt.Println("Connection closed")
			return
		}
		val, _, err := RespParser(buff)
		if err != nil {
			fmt.Println("Parse error:", err)
			connection.Write([]byte("-ERR parse error\r\n"))
			continue
		}
		if val.typ != '*' || len(val.array) == 0 {
			connection.Write([]byte("-ERR expected array\r\n"))
			continue
		}
		cmd := strings.ToUpper(val.array[0].str)
		args := val.array[1:]

		handler, ok := commands[cmd]
		if !ok {
			connection.Write([]byte("-ERR unknown command\r\n"))
			continue
		}
		connection.Write([]byte(handler(args)))
	}
}

func handlePing(args []RespValue) string {
	return "+PONG\r\n"
}

func handleEcho(args []RespValue) string {
	if len(args) < 1 {
		return "-ERR wrong number of arguments\r\n"
	}
	s := args[0].str
	return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
}

func RespParser(buff []byte) (RespValue, int, error) {
	if len(buff) == 0 {
		return RespValue{}, 0, fmt.Errorf("empty input")
	}
	typ := buff[0]
	switch typ {
	case '+', '-':
		end := findCRLF(buff, 1)
		if end == -1 {
			return RespValue{}, 0, fmt.Errorf("missing CRLF")
		}
		return RespValue{typ: typ, str: string(buff[1:end])}, end + 2, nil

	case ':':
		end := findCRLF(buff, 1)
		if end == -1 {
			return RespValue{}, 0, fmt.Errorf("missing CRLF")
		}
		n, err := parseInteger(buff[1:end])
		if err != nil {
			return RespValue{}, 0, err
		}
		return RespValue{typ: typ, integer: n}, end + 2, nil

	case '$': //bulk string
		end := findCRLF(buff, 1)
		if end == -1 {
			return RespValue{}, 0, fmt.Errorf("missing CRLF")
		}
		length, err := parseInteger(buff[1:end])
		if err != nil {
			return RespValue{}, 0, err
		}
		if length == -1 {
			return RespValue{typ: typ, str: ""}, end + 2, nil // null bulk string
		}
		start := end + 2
		newEnd := int(start) + length
		if int(len(buff)) < +2 {
			return RespValue{}, 0, fmt.Errorf("incomplete bulk string")
		}
		return RespValue{typ: typ, str: string(buff[start:newEnd])}, int(newEnd + 2), nil

	case '*': // array
		end := findCRLF(buff, 1)
		if end == -1 {
			return RespValue{}, 0, fmt.Errorf("missing CRLF")
		}
		lenArr, err := parseInteger(buff[1:end])
		if err != nil {
			return RespValue{}, 0, err
		}
		pos := end + 2
		elements := make([]RespValue, 0, lenArr)
		for i := 0; i < lenArr; i++ {
			val, consumed, err := RespParser(buff[pos:])
			if err != nil {
				return RespValue{}, 0, err
			}
			elements = append(elements, val)
			pos += consumed
		}
		return RespValue{typ: typ, array: elements}, pos, nil

	default:
		return RespValue{}, 0, fmt.Errorf("unknown type byte: %c", typ)
	}
}

func findCRLF(buff []byte, start int) int {
	for i := start; i < len(buff)-1; i++ {
		if buff[i] == '\r' && buff[i+1] == '\n' {
			return i
		}
	}
	return -1
}

func parseInteger(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, fmt.Errorf("empty integer")
	}
	neg := false
	start := 0
	if b[0] == '-' {
		neg = true
		start = 1
	}
	var n int
	for _, c := range b[start:] {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid integer byte: %c", c)
		}
		n = n*10 + int(c-'0')
	}
	if neg {
		n = -n
	}
	return n, nil
}
