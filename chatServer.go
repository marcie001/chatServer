// Thanks to https://gist.github.com/drewolson/3950226
package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"unicode/utf8"
)

type Client struct {
	name    string
	in      chan string
	out     chan string
	scanner *bufio.Scanner
	writer  *bufio.Writer
	conn    net.Conn
	room    *ChatRoom
}

func (client *Client) NameLabel() string {
	return fmt.Sprintf("%s: ", client.name)
}

func (client *Client) Read() {
	for client.scanner.Scan() {
		line := client.scanner.Text()
		Execute(client, line)
	}
	if err := client.scanner.Err(); err != nil {
		log.Print(err)
	}
}

func (client *Client) Write() {
	for data := range client.out {
		_, err := client.writer.WriteString(data)
		if err != nil {
			log.Print(err)
		}
		if strings.HasSuffix(data, "\n") {
			client.writer.Flush()
		}
	}
}

func (client *Client) Listen() {
	go client.Read()
	go client.Write()
}

func (client *Client) Close() {
	client.conn.Close()
	close(client.out)
	close(client.in)
}

func NewClient(conn net.Conn, room *ChatRoom) *Client {
	writer := bufio.NewWriter(conn)
	scanner := bufio.NewScanner(conn)
	scanner.Split(bufio.ScanLines)
	name := receiveAnswer("What's your name?", conn)
	client := &Client{
		name:    name,
		in:      make(chan string),
		out:     make(chan string),
		scanner: scanner,
		writer:  writer,
		conn:    conn,
		room:    room,
	}
	client.Listen()
	return client
}

type ChatRoom struct {
	clients []*Client
	joins   chan net.Conn
	leaves  chan *Client
	in      chan string
	out     chan string
}

func (chatRoom *ChatRoom) BroadCast(data string) {
	for _, c := range chatRoom.clients {
		c.out <- data
	}
}

func (chatRoom *ChatRoom) Join(conn net.Conn) {
	client := NewClient(conn, chatRoom)
	chatRoom.clients = append(chatRoom.clients, client)
	go func() {
		for {
			data, ok := <-client.in
			if !ok {
				return
			}
			chatRoom.in <- data
		}
	}()
	chatRoom.BroadCast(fmt.Sprintf("%s joined.\n", client.name))
}

func (chatRoom *ChatRoom) Leave(client *Client) {
	_, err := client.writer.WriteString("Bye.\n")
	if err != nil {
		log.Print(err)
	}
	client.writer.Flush()
	for i, c := range chatRoom.clients {
		if c == client {
			if i != len(chatRoom.clients)-1 {
				chatRoom.clients = append(chatRoom.clients[:i], chatRoom.clients[i+1:]...)
			} else {
				chatRoom.clients = chatRoom.clients[:i]
			}
			client.Close()
			chatRoom.BroadCast(fmt.Sprintf("%s left.\n", client.name))
			break
		}
	}
}

func (chatRoom *ChatRoom) Listen() {
	go func() {
		for {
			select {
			case data := <-chatRoom.in:
				chatRoom.BroadCast(data)
			case conn := <-chatRoom.joins:
				chatRoom.Join(conn)
			case client := <-chatRoom.leaves:
				chatRoom.Leave(client)
			}
		}
	}()
}

func NewChatRoom() *ChatRoom {
	chatRoom := &ChatRoom{
		clients: make([]*Client, 0, 4),
		joins:   make(chan net.Conn),
		leaves:  make(chan *Client),
		in:      make(chan string),
		out:     make(chan string),
	}
	chatRoom.Listen()
	return chatRoom
}

func receiveAnswer(question string, conn net.Conn) string {
	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)
	for {
		writer.WriteString(question)
		writer.WriteString(" ")
		writer.Flush()
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Print(err)
			continue
		}
		if line = strings.Trim(line, " \r\n"); len(line) > 0 {
			return line
		}
	}
}

func Execute(client *Client, line string) error {
	command, args, err := parse(line)
	if err != nil {
		return err
	}

	f, err := toFunc(command)
	if err != nil {
		return err
	}

	f(client, args...)
	return nil
}

// line to command and its args.
func parse(line string) (string, []string, error) {
	words := make([]string, 0, 8)
	scanner := bufio.NewScanner(strings.NewReader(line))
	scanner.Split(ScanWordsCustom)
	for scanner.Scan() {
		words = append(words, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", nil, err
	}
	if len(words) == 0 {
		return "", nil, errors.New("no input.")
	}

	if !strings.HasPrefix(words[0], ".") {
		return ".msg", []string{line}, nil
	}
	if len(words) > 1 {
		return words[0], words[1:], nil
	}
	return words[0], make([]string, 0), nil
}

func ScanWordsCustom(data []byte, atEOF bool) (advance int, token []byte, err error) {
	// skip leading separators
	start := 0
	inDoubleQuote := false
	for width := 0; start < len(data); start += width {
		var r rune
		r, width = utf8.DecodeRune(data[start:])
		if !isSeparator(r) {
			if r == '"' {
				inDoubleQuote = true
				start += width
			}
			break
		}
	}

	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	var (
		prevr     rune
		prevwidth int
	)
	for width, i := 0, start; i < len(data); i += width {
		var r rune
		r, width = utf8.DecodeRune(data[i:])
		if !inDoubleQuote && isSeparator(r) {
			if prevr == '"' {
				return i + width, data[start : i-prevwidth], nil
			}
			return i + width, data[start:i], nil
		}
		if r == '"' {
			inDoubleQuote = !inDoubleQuote
		}
		prevr = r
		prevwidth = width
	}

	if atEOF && len(data) > start {
		if prevr == '"' {
			return len(data), data[start : len(data)-prevwidth], nil
		}
		return len(data), data[start:], nil
	}
	return 0, nil, nil
}

func isSeparator(r rune) bool {
	switch r {
	case ' ', '\t':
		return true
	}
	return false
}

type Command func(*Client, ...string)

func KickOut(client *Client, clientNames ...string) {
	for _, targetName := range clientNames {
		for _, c := range client.room.clients {
			if strings.EqualFold(targetName, c.name) {
				client.room.leaves <- c
			}
		}
	}
}

func Quit(client *Client, args ...string) {
	client.room.leaves <- client
}

func Message(client *Client, args ...string) {
	client.in <- client.name
	client.in <- ": "
	for _, s := range args {
		client.in <- s
	}
	client.in <- "\n"
}

func DM(client *Client, args ...string) {
	if len(args) < 2 {
		return
	}
	for _, targetName := range args[:len(args)-1] {
		for _, c := range client.room.clients {
			if strings.EqualFold(targetName, c.name) {
				c.out <- client.name
				c.out <- "(DM): "
				c.out <- args[len(args)-1]
				c.out <- "\n"
			}
		}
	}
}

// return a command func for commandName.
// If no command was found, return error.
func toFunc(commandName string) (Command, error) {
	switch strings.ToLower(commandName) {
	default:
		return nil, errors.New("invalid command.")
	case ".quit":
		return Quit, nil
	case ".kick":
		return KickOut, nil
	case ".dm":
		return DM, nil
	case ".msg":
		return Message, nil
	}
}

func main() {
	logfile, err := os.OpenFile("/tmp/tcpserver.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logfile.Close()
	log.SetOutput(logfile)

	listener, err := net.Listen("tcp", ":8800")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	log.Print("Server started!!")

	chatRoom := NewChatRoom()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go chatRoom.Join(conn)
	}
}
