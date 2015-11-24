package main

import (
	"bufio"
	"io"
	"log"
	"net"
)

import "os"
import "syscall"
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
		return nil, errors.New("File is not a serial port")
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
func handleSerial(port io.ReadWriteCloser, chin chan string, chout chan string) {
	// The write thread
	go func(port io.ReadWriteCloser, ch chan string) {
		io.WriteString(port, <-ch)
	}(port, chin)
	// The read thread
	go func(port io.ReadWriteCloser, ch chan string) {
		scanner := bufio.NewScanner(port)
		if scanner.Scan() {
			ch <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}(port, chout)
}

/**
 * Main TCP handling function
 */
func handleConnection(conn net.Conn, chin chan string, chout chan string) {
	// the read thread
	go func(c net.Conn, ch chan string) {
		io.Copy(conn, conn)
		conn.Close()
	}(conn, chin)
	// the write thread
	go func(c net.Conn, ch chan string) {
		io.WriteString(conn, <-ch)
		conn.Close()
	}(conn, chout)
}

/**
 * Main() - Program entry function
 */
func main() {
	// make the comms channel object
	chin := make(chan string, 1)
	chout := make(chan string, 1)
	// open the serial port
	port, err := openSerial()
	if err != nil {
		log.Fatal(err)
	}
	// listen to the serial port
	handleSerial(port, chin, chout)
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
		go handleConnection(conn, chin, chout)
	}
}
