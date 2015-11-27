package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
)

import "os"
import "syscall"

// #include <termios.h>
// #include <unistd.h>
import "C"

/**
 * Open the serial port setting the baud etc.
 */
func openSerial() (io.ReadWriteCloser, error) {
	file, err :=
		os.OpenFile(
			"/dev/ttyUSB0",
			syscall.O_RDWR|syscall.O_NOCTTY,
			0600)
	if err != nil {
		return nil, err
	}

	fd := C.int(file.Fd())
	if C.isatty(fd) == 0 {
		err := errors.New("File is not a serial port")
		return nil, err
	}

	var termios C.struct_termios
	_, err = C.tcgetattr(fd, &termios)
	if err != nil {
		return nil, err
	}

	var baud C.speed_t
	baud = C.B115200
	_, err = C.cfsetispeed(&termios, baud)
	if err != nil {
		return nil, err
	}
	_, err = C.cfsetospeed(&termios, baud)
	if err != nil {
		return nil, err
	}
	return file, nil
}

/**
 * Main serial handling function
 */
func serialReader(port io.ReadWriteCloser, serin chan string) {
	/*
		// The write thread
		go func(port io.ReadWriteCloser, ch chan string) {
			for {
				s := <-ch
				_, err := io.WriteString(port, s)
				if err != nil {
					fmt.Println(err)
					break
				}
			}
			port.Close()
		}(port, chin)
	*/
	// The read thread
	scanner := bufio.NewScanner(port)
	for {
		if scanner.Scan() {
			s := fmt.Sprintln(scanner.Text())
			fmt.Print(s)
			serin <- s
		}
		if err := scanner.Err(); err != nil {
			fmt.Println(err)
			break
		}
	}
	port.Close()
}

/**
 * Main TCP handling function
 */
func handleConnection(conn net.Conn, chin chan string, chout chan string) {
	// the read thread
	go func(c net.Conn, ch chan string) {
		scanner := bufio.NewScanner(conn)
		for {
			if scanner.Scan() {
				fmt.Println(scanner.Text())
				ch <- fmt.Sprintln(scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				fmt.Println(err)
				break
			}
		}
		conn.Close()
	}(conn, chin)
	// the write thread
	go func(c net.Conn, ch chan string) {
		for {
			_, err := io.WriteString(conn, <-ch)
			if err != nil {
				fmt.Println(err)
				break
			}
		}
		conn.Close()
	}(conn, chout)
}

/**
 * Due to vagaries of chan we need to add to each client queue
 *
 */
func fanOut(serin chan string, clients []chan string, reset chan struct{}) {
	var str string
Loop:
	for {
		select {
		case str = <-serin:
			for i := range clients {
				clients[i] <- str
			}
		case <-reset:
			break Loop
		}
	}
}

/**
 * Main() - Program entry function
 */
func main() {
	serout := make(chan string, 0) // serial write channel
	serin := make(chan string, 0)  // serial read channel
	done := make(chan struct{}, 0) // finish signal channel
	clients := make([]chan string, 1, 15)

	// open the serial port
	port, err := openSerial()
	if err != nil {
		log.Fatal(err)
	}
	serialReader(port, serin)
	// listen on 1812 for connections
	ln, err := net.Listen("tcp", ":1812")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		// maximum 10 clients
		if len(clients) >= 10 {
			conn.Close()
			continue
		}
		chout := make(chan string, 0)
		clients := append(clients, chout)
		for _ = range clients {
			done <- struct{}{}
		}
		go fanOut(serin, clients, done)
		go handleConnection(conn, serout, chout)
	}
}
