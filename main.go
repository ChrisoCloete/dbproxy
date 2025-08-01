package main

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
)

// TunnelConfig holds the configuration for a single tunnel
type TunnelConfig struct {
	ListenerAddress string
	ServerHost      string
	SSHUser         string
	SSHKeyFile      string
	SSHPassword     string
	DBHost          string
}

func main() {
	pwd, _ := os.Getwd()
	_ = godotenv.Load(pwd + "/.env")

	// Parse tunnel configurations
	tunnelConfigs := parseTunnelConfigs()
	if len(tunnelConfigs) == 0 {
		fmt.Println("No tunnel configurations found")
		return
	}

	// Wait group to keep the main function running
	var wg sync.WaitGroup

	// Start a goroutine for each tunnel
	for i, config := range tunnelConfigs {
		wg.Add(1)
		go func(index int, cfg TunnelConfig) {
			defer wg.Done()
			runTunnel(index, cfg)
		}(i, config)
	}

	// Wait for all tunnels to complete (they won't unless there's an error)
	wg.Wait()
}

// parseTunnelConfigs parses multiple tunnel configurations from environment variables
func parseTunnelConfigs() []TunnelConfig {
	var configs []TunnelConfig

	// Check for numbered configurations first (LISTENER_ADDRESS_1, LISTENER_ADDRESS_2, etc.)
	for i := 1; ; i++ {
		suffix := strconv.Itoa(i)
		listenerAddress := os.Getenv("LISTENER_ADDRESS_" + suffix)
		if listenerAddress == "" {
			break
		}

		configs = append(configs, TunnelConfig{
			ListenerAddress: listenerAddress,
			ServerHost:      os.Getenv("SERVER_HOST_" + suffix),
			SSHUser:         os.Getenv("SHH_USER_" + suffix),
			SSHKeyFile:      os.Getenv("SHH_KEY_FILE_" + suffix),
			SSHPassword:     os.Getenv("SSH_PASSWORD_" + suffix),
			DBHost:          os.Getenv("DB_HOST_" + suffix),
		})
	}

	// If no numbered configurations found, try the original format
	if len(configs) == 0 && os.Getenv("LISTENER_ADDRESS") != "" {
		configs = append(configs, TunnelConfig{
			ListenerAddress: os.Getenv("LISTENER_ADDRESS"),
			ServerHost:      os.Getenv("SERVER_HOST"),
			SSHUser:         os.Getenv("SHH_USER"),
			SSHKeyFile:      os.Getenv("SHH_KEY_FILE"),
			SSHPassword:     os.Getenv("SSH_PASSWORD"),
			DBHost:          os.Getenv("DB_HOST"),
		})
	}

	return configs
}

// runTunnel starts a tunnel with the given configuration
func runTunnel(index int, config TunnelConfig) {
	slog.Info("Starting tunnel", "index", index, "listener", config.ListenerAddress, "dbHost", config.DBHost)

	listener, err := net.Listen("tcp", config.ListenerAddress)
	if err != nil {
		fmt.Printf("Failed to listen on %s: %v\n", config.ListenerAddress, err)
		return
	}
	defer listener.Close()

	for {
		// Accept new connections from the application
		appConn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Failed to accept connection on %s: %v\n", config.ListenerAddress, err)
			continue
		}

		// Establish SSH tunnel and forward connection
		dbConn, err := sshTunnel(config)
		if err != nil {
			fmt.Printf("Failed to establish SSH tunnel to %s: %v\n", config.DBHost, err)
			appConn.Close()
			continue
		}

		// Forward data between the application and the database
		go forwardTraffic(appConn, dbConn)
	}
}

func sshTunnel(config TunnelConfig) (*net.Conn, error) {
	// Configure SSH client config
	sshConfig := &ssh.ClientConfig{
		User:            config.SSHUser,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Adjust as needed
	}

	// Check if a key file is provided
	if config.SSHKeyFile != "" {
		// Try to use it as a key file first
		buffer, err := os.ReadFile(config.SSHKeyFile)
		if err == nil {
			key, err := ssh.ParsePrivateKey(buffer)
			if err == nil {
				// Successfully parsed key file, use public key authentication
				sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(key))
				slog.Info("Using public key authentication", "keyFile", config.SSHKeyFile)
			} else if config.SSHPassword != "" {
				// Failed to parse key file, but password is provided
				sshConfig.Auth = append(sshConfig.Auth, ssh.Password(config.SSHPassword))
				slog.Info("Using password authentication")
			} else {
				// No password provided, try using key file as password (for backward compatibility)
				sshConfig.Auth = append(sshConfig.Auth, ssh.Password(config.SSHKeyFile))
				slog.Info("Using password authentication (from key file)")
			}
		} else if config.SSHPassword != "" {
			// Failed to read key file, but password is provided
			sshConfig.Auth = append(sshConfig.Auth, ssh.Password(config.SSHPassword))
			slog.Info("Using password authentication")
		} else {
			// No password provided, try using key file as password (for backward compatibility)
			sshConfig.Auth = append(sshConfig.Auth, ssh.Password(config.SSHKeyFile))
			slog.Info("Using password authentication (from key file)")
		}
	} else if config.SSHPassword != "" {
		// No key file provided, but password is provided
		sshConfig.Auth = append(sshConfig.Auth, ssh.Password(config.SSHPassword))
		slog.Info("Using password authentication")
	} else {
		return nil, fmt.Errorf("No authentication method provided")
	}

	// Connect to bastion host
	conn, err := ssh.Dial("tcp", config.ServerHost, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to ssh host: %w", err)
	}

	// Establish tunnel to the database
	dbConn, err := conn.Dial("tcp", config.DBHost)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to the database through SSH tunnel %s: %w", config.DBHost, err)
	}
	slog.Info("Tunnel established to database", "dbHost", config.DBHost)
	return &dbConn, nil
}

func forwardTraffic(appConn net.Conn, dbConn *net.Conn) {
	defer appConn.Close()
	defer (*dbConn).Close()
	slog.Info("Forwarding traffic for connection", "remoteAddr", appConn.RemoteAddr().String())
	errChan := make(chan error, 2)
	go func() {
		_, err := io.Copy(appConn, *dbConn)
		errChan <- err
	}()
	_, err := io.Copy(*dbConn, appConn)
	errChan <- err
	for i := 0; i < 2; i++ {
		if err := <-errChan; err != nil {
			slog.Error("Traffic forwarding error", "error", err)
		}
	}
}
