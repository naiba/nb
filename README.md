# NB

开发者终端增强工具，让 `git/ssh/scp/rsync` 更顺手，内置区块链开发工具。

## 安装

```sh
go install github.com/naiba/nb@latest
```

## 核心功能

### 多账号 Git/SSH

一条命令切换账号，告别 `~/.ssh/config` 地狱：

```sh
nb -gu work git push origin main    # 用 work 账号推送
nb -gu personal git commit -m "..."  # 用 personal 账号提交
nb -p proxy -ss server ssh           # 通过代理连接服务器
```

### 区块链靓号生成

多线程生成指定前缀/后缀的钱包地址：

```sh
nb ethereum vanity -p dead -s beef   # ETH: 0xdead...beef
nb solana vanity -p Sol              # Solana: Sol...
nb tron vanity -p T9y                # TRON: T9y...
```

### Solana 三明治攻击检测

```sh
nb solana check-sandwich-attack --rpc https://api.mainnet-beta.solana.com --tx <signature>
```

### Claude Code Guard

自动化处理 Claude Code CLI 的简单确认，TUI 界面可随时接管：

```sh
nb ccguard "Help me refactor this code"
```

## 配置

首次运行 `nb` 会生成配置文件 `~/.config/nb.yaml`，配置你的代理、SSH 主机和 Git 账号：

```yaml
git:
  work:
    email: work@company.com
    name: Your Name
    ssh_prikey: ~/.ssh/id_work
  personal:
    email: personal@gmail.com
    name: Your Name
    ssh_prikey: ~/.ssh/id_personal

ssh:
  server1:
    host: 192.168.1.100
    login: root
    prikey: ~/.ssh/id_rsa

proxy:
  my-proxy:
    socks: 127.0.0.1:1080
```

## 更多命令

```sh
nb -h                    # 查看所有命令
nb ethereum -h           # 查看 Ethereum 子命令
nb forge export-abi      # 导出合约 ABI
nb convert --from hex --to base64 0xdeadbeef
nb update                # 更新到最新版本
```

## License

MIT - [naiba](https://github.com/naiba)
