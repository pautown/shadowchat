# Shadowchat (Paul's Version)

- Self-hosted, noncustodial and minimalist Monero (XMR) and Solana (SOL) and Ethereum (ETH) and Hex (HEX) livestream donation system written in Go.
- Provides an admin view page to see donations with corresponding comments.
- Provides notification methods usable in OBS with an HTML page.

To see a working instance of shadowchat, see [pay.paul.town](pay.paul.town).

# Installation

1. ```apt install golang```
2. ```git clone https://git.sr.ht/~anon_/shadowchat```
3. ```cd shadowchat```
4. ```go install github.com/skip2/go-qrcode@latest```
5. edit ```config.json```
6. ```go run main.go```

A webserver at 127.0.0.1:8900 is running. Pressing the pay button will result in a 500 Error if the `monero-wallet-rpc`
is not running.
This is designed to be run on a cloud server with nginx proxypass for TLS.

# Monero Setup

1. Generate a view only wallet using the `monero-wallet-gui` from getmonero.org. Preferably with no password
2. Copy the newly generated `walletname_viewonly` and `walletname_viewonly.keys` files to your VPS
3. Download the `monero-wallet-rpc` binary that is bundled with the getmonero.org wallets.
4. Start the RPC
   wallet: `monero-wallet-rpc --rpc-bind-port 28088 --daemon-address https://xmr-node.cakewallet.com:18081 --wallet-file /opt/wallet/walletname_viewonly --disable-rpc-login --password ""`

# Usage

- Visit 127.0.0.1:8900/view to view your superchat history
- Visit 127.0.0.1:8900/alert?auth=adminadmin to see notifications
- The default username is `admin` and password `adminadmin`. Change these in `main.go`
- Edit web/index.html and web/style.css to customize your front page!

# OBS

- Add a Browser source in obs and point it to `https://example.com/alert?auth=adminadmin`

# Future plans

- Settings page for on-the-fly changes (minimum donation amount, hide all amounts, etc.)
- Admin page for easy nav
- Bootstrap UI for easy nav
- Integration with chatbot
- User games via donos (banning others from chatting)
- Rewriting donor CSV into DB
- Rewriting most of codebase
- Custom OBS notification page
- Better read-me
- User login/password system based on real DB and encryption

# License

GPLv3

### Origin

This comes from [https://git.sr.ht/~anon_/shadowchat](https://git.sr.ht/~anon_/shadowchat) and is not Paul's original
work.


