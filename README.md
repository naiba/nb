# 终端助手

My terminal assistant

## :fried_egg: Quick Start

```sh
go install github.com/naiba/nb@latest
nb help
```

Append this line to your `.zshrc`

```sh
alias NB_CONFIG_PATH=/path-to-nb.yaml nb

# Also you can put your alias etc. to `nb` snippet then source them
source <(/path-to-go/bin/nb --config-path /path-to-nb.yaml print-snippet profile)

# Print banner
nb banner
```
