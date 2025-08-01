# dbproxy
Small project I spun up to allow devs in my team to easily connect to our staging db.

This project is far from perfect and I might still work on it.
For now it serves its purpose.
Use at own risk.

## Configuration

You need to create an .env file similar to the .env.example for your setup.

### Single Tunnel Configuration
For a single tunnel, you can use the legacy format:
```
LISTENER_ADDRESS=localhost:5432
SERVER_HOST=host:port
SHH_USER=user
# You can use either a key file or a password for authentication
# If both are provided, the key file will be tried first
SHH_KEY_FILE=path/to/key/file
SSH_PASSWORD=your_password
DB_HOST=db_host:db_port
```

### Multiple Tunnel Configuration
The application now supports running multiple tunnels simultaneously. To configure multiple tunnels, use the numbered format:
```
# First tunnel
LISTENER_ADDRESS_1=localhost:5432
SERVER_HOST_1=host1:port
SHH_USER_1=user1
# You can use either a key file or a password for authentication
# If both are provided, the key file will be tried first
SHH_KEY_FILE_1=path/to/key/file1
SSH_PASSWORD_1=your_password1
DB_HOST_1=db_host1:db_port1

# Second tunnel
LISTENER_ADDRESS_2=localhost:5433
SERVER_HOST_2=host2:port
SHH_USER_2=user2
# You can use either a key file or a password for authentication
# If both are provided, the key file will be tried first
SHH_KEY_FILE_2=path/to/key/file2
SSH_PASSWORD_2=your_password2
DB_HOST_2=db_host2:db_port2
```

You can add as many tunnels as needed by incrementing the number suffix.
