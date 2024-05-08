package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"os"
)

func main() {
	pwd, _ := os.Getwd()
	_ = godotenv.Load(pwd + "/.env")

	listener, err := net.Listen("tcp", os.Getenv("LISTENER_ADDRESS"))
	if err != nil {
		fmt.Println("Failed to listen:", err)
		return
	}
	defer listener.Close()
	for {
		// Accept new connections from the application
		appConn, err := listener.Accept()
		if err != nil {
			fmt.Println("Failed to accept connection:", err)
			continue
		}
		// Establish SSH tunnel and forward connection
		dbConn, err := sshTunnel(os.Getenv("SERVER_HOST"), os.Getenv("SHH_USER"), os.Getenv("SHH_KEY_FILE"), os.Getenv("DB_HOST"))
		if err != nil {
			fmt.Println("Failed to establish SSH tunnel:", err)
			appConn.Close()
			continue
		}
		// Forward data between the application and the database
		go forwardTraffic(appConn, dbConn)
	}
}

func sshTunnel(host, user, keyFile, dbHost string) (*net.Conn, error) {
	buffer, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("Failed read key file: %w", err)
	}
	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse key file: %w", err)
	}
	authMethod := ssh.PublicKeys(key)
	// Configure SSH client config
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Adjust as needed
	}
	// Connect to bastion host
	conn, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to ssh host: %w", err)
	}
	// Establish tunnel to the database
	dbConn, err := conn.Dial("tcp", dbHost)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to the database through SSH tunnel: %w", err)
	}
	return &dbConn, nil
}

func forwardTraffic(appConn net.Conn, dbConn *net.Conn) {
	defer appConn.Close()
	defer (*dbConn).Close()
	// Copy data between the application connection and the database connection
	go io.Copy(appConn, *dbConn)
	io.Copy(*dbConn, appConn)
}
