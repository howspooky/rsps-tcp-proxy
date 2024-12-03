package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

var connFilter *ConnFilter
var s Stats = *NewStats()

const (
	BAD_SESSION_ID    = 10
	REJECTED_SESSION  = 11
	ATTEMPTS_EXCEEDED = 16
	UNABLE_TO_CONNECT = 8
	MALFORMED_LOGIN   = 27
	NO_RESPONSE       = 28
)

type Config struct {
	BindIP      string `env:"BIND_IP" env-required:""`
	BindPort    int    `env:"BIND_PORT" env-required:""`
	ServerIP    string `env:"SERVER_IP" env-required:""`
	ServerPort  int    `env:"SERVER_PORT" env-required:""`
	MaxAttempts int    `env:"MAX_ATTEMPTS" env-default:"5"`
}

func main() {
	config := Config{}
	err := cleanenv.ReadEnv(&config)
	if err != nil {
		log.Fatal("error loading config", err)
	}

	slog.Info("starting server", slog.String("ip", config.BindIP), slog.Int("port", config.BindPort))

	connFilter = NewConnFilter(config.MaxAttempts)
	go tick()

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", config.BindIP, config.BindPort))
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleNewConnection(conn, config)
	}
}

func tick() {
	ticker := time.NewTicker(30 * time.Second)

	for range ticker.C {
		s.Print()
		s.Reset()

		connFilter.Tick()
	}
}

func writeError(conn net.Conn, errorCode int) {
	conn.Write([]byte{byte(errorCode)})
	// artificial sleep to allow the client to process the request
	time.Sleep(1 * time.Second)
	conn.Close()
}

func handleNewConnection(conn net.Conn, config Config) {
	addr := conn.RemoteAddr().(*net.TCPAddr)
	ip := addr.IP

	slog.Info("connection attempt", slog.Any("addr", ip))

	ipv4 := ip.To4()
	if ipv4 == nil {
		writeError(conn, REJECTED_SESSION)
		return
	}

	ipId := binary.BigEndian.Uint32(ipv4)

	attempts := connFilter.RecordAndGetConnAttempts(ipId)
	if attempts >= config.MaxAttempts {
		slog.Warn("too many connection attempts", slog.Any("ip", ip), slog.Int("attempts", attempts))
		writeError(conn, ATTEMPTS_EXCEEDED)
		return
	}

	handleRequest(conn, ipv4.String(), config.ServerIP, config.ServerPort)
}

func handleRequest(conn net.Conn, playerIP string, serverIP string, serverPort int) {
	s.IncrementConnectionsOpened()

	clientReader := bufio.NewReader(conn)

	buffer := make([]byte, 10)
	n, err := clientReader.Read(buffer)
	if err != nil {
		slog.Warn("Error reading first byte", slog.String("ip", playerIP), slog.Any("err", err))
		writeError(conn, REJECTED_SESSION)
		return
	}

	if n > 1 {
		slog.Warn("Received more bytes than we should", slog.String("ip", playerIP), slog.Int("total bytes", n))
		writeError(conn, MALFORMED_LOGIN)
		return
	}

	if buffer[0] != 14 {
		slog.Warn("Wrong first byte", slog.String("ip", playerIP), slog.Int("opcode", int(buffer[0])))
		writeError(conn, MALFORMED_LOGIN)
		return
	}

	ip := net.ParseIP(playerIP)
	if ip == nil || ip.To4() == nil {
		writeError(conn, REJECTED_SESSION)
		slog.Warn("Invalid IPv4 address", slog.String("ip", playerIP))
		return
	}

	ipv4 := ip.To4()
	byteSlice := append(ipv4, buffer[:1]...)

	serverConn, err := connectToServer(serverIP, serverPort)
	if err != nil {
		writeError(conn, NO_RESPONSE)
		slog.Warn("Error connecting to server", slog.String("ip", playerIP), slog.Any("err", err))
		return
	}

	clientWriter := bufio.NewWriter(conn)
	serverReader := bufio.NewReader(serverConn)
	serverWriter := bufio.NewWriter(serverConn)
	ctx, cancel := context.WithCancel(context.Background())
	s.IncrementConnections()

	var closeOnce sync.Once
	closeConnections := func() {
		closeOnce.Do(func() {
			slog.Info("Closing connections", slog.String("ip", playerIP))
			conn.Close()
			serverConn.Close()
			cancel()

			s.DecrementConnections()
		})
	}

	if _, err := serverWriter.Write(byteSlice); err != nil {
		slog.Warn("Error writing to server", slog.String("ip", playerIP), slog.Any("err", err))
		closeConnections()
		return
	}

	serverWriter.Flush()
	slog.Info("connection success", slog.String("ip", playerIP))

	go receiveAndWrite(true, ctx, clientReader, serverWriter, closeConnections)
	go receiveAndWrite(false, ctx, serverReader, clientWriter, closeConnections)
}

func receiveAndWrite(isClient bool, ctx context.Context, reader *bufio.Reader, writer *bufio.Writer, closeConnections func()) {
	buffer := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n, err := reader.Read(buffer)
			if err != nil {
				closeConnections()
				return
			}
			if isClient {
				s.AddClientBytesRead(int64(n))
			} else {
				s.AddServerBytesRead(int64(n))
			}
			if _, err := writer.Write(buffer[:n]); err != nil {
				closeConnections()
				return
			}
			if err := writer.Flush(); err != nil {
				closeConnections()
				return
			}
		}
	}
}

func connectToServer(ip string, port int) (net.Conn, error) {
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.Dial("tcp", fmt.Sprintf("%s:%d", ip, port))

	if err != nil {
		return nil, err
	}

	return conn, nil
}
