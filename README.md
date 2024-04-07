# NB Terminal assistant

:knife: My terminal assistant, enhanced git, ssh, scp ... commands.

## Installation

Config file example [./nb.yaml](./nb.yaml)

```sh
go install github.com/naiba/nb@latest
nb # prepare config file to config path
```

## Usage

```sh
# Load alias etc. can append to .profile/.zshrc
source <(/path-to-go/bin/nb --config-path /path-to-nb.yaml print-snippet profile)

# Print banner, can append to .profile/.zshrc
nb print-banner

# Connecting to SSH server via socks proxy
nb -p rpi-socks -ss github ssh

# Copy remote files via socks proxy
nb -p rpi-socks -ss github scp -v -r ./nb.yaml remote:/

# Specify an account to perform git operations
# This commit will be signed by naiba even if you are in the git repo of another account.
nb -gu naiba git commit -a -m "test"
```
