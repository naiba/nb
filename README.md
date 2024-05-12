# NB Terminal assistant

:knife: My terminal assistant, enhanced git, ssh, scp ... commands.

## Installation

Config file example [./nb.yaml](./nb.yaml), put it to `~/.config/nb.yaml`.

```sh
go install github.com/naiba/nb@latest
nb # prepare config file to config path
```

## Usage

```sh
# Print banner, can append to .profile/.zshrc
nb print-banner

# Cloudflate Batch DNS Record Management GUI
nb cloudflare

# Load alias etc. can append to .profile/.zshrc
source <(/path-to-go/bin/nb print-snippet profile)

# Connecting to SSH server via socks proxy
nb -p rpi-socks -ss github ssh

# Copy remote files via socks proxy
nb -p rpi-socks -ss github scp -v -r ./nb.yaml remote:/

# Specify an account to perform git operations
# This commit will be signed by naiba even if you are in the git repo of another account.
nb -gu naiba git commit -a -m "test"

# Solidity crete2 vanity address
nb solidity create2vanity -p 0xb300000 -d 0x13b0D85CcB8......CA081AE9beF2 --sp com.example. \
    --cb 0x60806001600160401b03601f6......dccf7284c10517a35c6 \
    --ca address:0xEe7b429Ea01......D4e95D5D24AE8 --ca address:0x57FE1CB49......d821e5e95
```
