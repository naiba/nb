# NB Terminal Assistant

:knife: A powerful terminal assistant with enhanced git, ssh, scp commands and blockchain development tools.

## Installation

```sh
go install github.com/naiba/nb@latest
nb # initialize and prepare config file
```

Config file example: [./nb.yaml](./nb.yaml)
Default location: `~/.config/nb.yaml`

## Features

### Core Commands
- **Enhanced Git** - Execute git operations with specific accounts and configurations
- **Enhanced SSH/SCP** - Connect through proxy servers with simplified syntax
- **Enhanced Rsync/NC** - Improved file sync and network tools
- **Cloudflare** - Batch DNS record management GUI
- **GitHub** - GitHub API helpers
- **Flutter** - Enhanced flutter command wrapper

### Blockchain Development Tools

#### Ethereum
- Vanity address generation (EOA, CREATE1, CREATE2)
- Timestamp to block number conversion
- Sandwich attack detection

#### Solana
- Vanity address generation
- Transaction decoding and retrieval
- Sandwich attack detection

#### Tron
- Vanity address generation
- RPC proxy server

#### Solidity/Forge
- ABI export utilities
- Contract unflattening
- Diamond standard upgrade tools

### Additional Tools
- **Anchor** - Anchor framework helper
- **Convert** - Multi-format converter (base58, base64, hex, bigint)
- **Run** - Command execution helper

## Usage Examples

### Setup & Configuration

```sh
# Print help
nb -h

# Print banner for terminal startup (add to .profile/.zshrc)
nb print-banner

# Print and source shell snippets (add to .profile/.zshrc)
source <(/path-to-go/bin/nb print-snippet profile)

# View current configuration
nb print-config
```

### Network & SSH

```sh
# Connect to SSH server via socks proxy
nb -p rpi-socks -ss github ssh

# Copy remote files via socks proxy
nb -p rpi-socks -ss github scp -v -r ./nb.yaml remote:/

# Enhanced rsync with proxy
nb -p my-proxy rsync -avz ./local/ remote:/backup/
```

### Git Operations

```sh
# Use specific git account for operations
# The commit will use the specified account's credentials
nb -gu naiba git commit -a -m "feat: add new feature"

# Push with specific account
nb -gu naiba git push origin main
```

### Blockchain - Ethereum

```sh
# Generate vanity EOA address
nb ethereum vanity -p 0xdead -p 0xbeef

# Generate vanity CREATE1 contract address (nonce=0)
nb ethereum vanity-create1 -p 0xcafe --deployer 0xYourAddress

# Generate vanity CREATE2 address
nb ethereum vanity-create2 -p 0xb300000 \
    -d 0x13b0D85CcB8......CA081AE9beF2 \
    --sp com.example. \
    --cb 0x60806001600160401b03601f6......dccf7284c10517a35c6 \
    --ca address:0xEe7b429Ea01......D4e95D5D24AE8

# Get block number by timestamp
nb ethereum timestamp-to-block-number --rpc https://eth.llamarpc.com --timestamp 1234567890

# Check sandwich attack
nb ethereum check-sandwich-attack --rpc https://eth.llamarpc.com --tx 0xYourTxHash
```

### Blockchain - Solana

```sh
# Generate vanity address
nb solana vanity -p Sol

# Decode transaction
nb solana decode-transaction --rpc https://api.mainnet-beta.solana.com --tx YourTxSignature

# Check sandwich attack
nb solana check-sandwich-attack --rpc https://api.mainnet-beta.solana.com --tx YourTxSignature
```

### Blockchain - Tron

```sh
# Generate vanity address
nb tron vanity -p T9y

# Start RPC proxy server
nb tron rpc-proxy --listen :8545 --target https://api.trongrid.io
```

### Solidity Development

```sh
# Export contract ABIs
nb forge export-abi -i ./contracts -o ./abis

# Unflatten a flattened contract
nb forge unflatten -i flattened.sol -o ./src

# Generate diamond upgrade parameters
nb forge diamond-upgrade --old ./old-facets --new ./new-facets
```

### Cloudflare

```sh
# Launch Cloudflare DNS management GUI
nb cloudflare
```

### Utilities

```sh
# Convert between formats
nb convert --from hex --to base64 0xdeadbeef
nb convert --from base58 --to hex SomeBase58String

# Update nb to latest version
nb update
```

## Global Options

```sh
--proxy, -p          # Use specified proxy server
--ssh-server, --ss   # Use specified SSH server
--git-user, --gu     # Use specified git account
--config-path, -c    # Specify config file path
--version, -v        # Show version
--help, -h           # Show help
```

## Configuration

Create `~/.config/nb.yaml` with your proxy servers, SSH hosts, and git accounts. See [nb.yaml](./nb.yaml) for examples.

## License

MIT

## Author

[naiba](https://github.com/naiba)
