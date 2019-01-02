# raiden-on-storj
Scripts for Integration of Raiden in the Storj Network

## Dependencies
- UNIX or OSX
- Go 1.11(.4)
- curl

# Install of onboarding tools
1. Clone Repo with: `git clone https://github.com/stefanbenten/raiden-on-storj`
2. Move into directory with: `cd raiden-on-storj` 
3. Execute: `go install ./...`

Now the binaries satellite and client should be available!

# Usage

Per Default a satellite is running and you are able to connect your Client to it by running:
`client`

It should generate a Ethereum Address/Wallet for you and open a webbrowser pointing to the Web Interface of the Client App.
