# portier-cli

<div align="center">
Remotely access all your machines through Portier CLI. It's easy, efficient and reliable. For more info, visit www.portier.dev!
<br>

## Forget networking, we love the web.

If complex network setup blocked you - search no more. Portier offers you remote connectivity with literally zero network setup. Access your remote machine from anywhere, no matter how it accesses the public internet. Don't care about techniques like NAT hole punching. Web-access to portier.dev (HTTP and Websockets) is the only requirement to use our services, even through proxies.

## Robust, reliable and lean.

With its automatic reconnect and advanced retransmission algorithms, your remote access works free from connection drops. Portier turns these events into a bit of latency, and then everything continues smoothly.
Portier-cli requires roughly 10MB of RAM to run, but is also capable of scaling up to handle thousands of parallel connections - if you need it.

## Secure, (don’t) trust us.

Portier uses TLS to secure your connections. And there’s no need to trust us: Portier-cli encrypts connections end to end (under development). Your data remains private.

## Low-latency, high throughput servers.

Portier uses a cloud infrastructure to forward messages between clients that handles high throughput with millisecond latencies. Working with rdp or ssh? Don’t worry about it, your clicks and key strokes will have a swift and fast response, just like you’re used to.

<br>
<br>
<img src="https://github.com/mh-dx/portier-cli/actions/workflows/test.yml/badge.svg" alt="drawing"/>
<img src="https://pkg.go.dev/badge/github.com/mh-dx/portier-cli.svg" alt="drawing"/>
<img src="https://img.shields.io/github/v/release/mh-dx/portier-cli" alt="drawing"/>
<img src="https://img.shields.io/docker/pulls/mh-dx/portier-cli" alt="drawing"/>
<img src="https://img.shields.io/github/downloads/mh-dx/portier-cli/total.svg" alt="drawing"/>
</div>

# Table of Contents
<!--ts-->
   * [portier-cli](#portier-cli)
   * [Install](#install)
   * [Setup](#setup)
   * [Project Layout](#project-layout)
   * [Makefile Targets](#makefile-targets)
   * [Contribute](#contribute)

# Install
## Prerequisites

Ensure you have `curl` or `wget` installed to download the binaries. For building from source, `make` and `gcc` (or an equivalent compiler) are required.

## Installing from Binaries

### macOS

For macOS, download the appropriate binary for your architecture:

- **Intel (x64):**
  ```bash
  curl -L https://github.com/mh-dx/portier-cli/releases/latest/download/portier-cli_<VERSION>_darwin_amd64.tar.gz | tar xz
  sudo mv portier-cli /usr/local/bin
  ```

- **Apple Silicon (ARM64):**
  ```bash
  curl -L https://github.com/mh-dx/portier-cli/releases/latest/download/portier-cli_<VERSION>_darwin_arm64.tar.gz | tar xz
  sudo mv portier-cli /usr/local/bin
  ```

### Linux

For Linux, binaries are available in `.deb`, `.rpm`, and `.tar.gz` formats. Choose the one appropriate for your system and architecture. Replace `<ARCH>` with your architecture, such as `amd64`, `arm64`, or `armv6`.

- **Debian-based systems (e.g., Ubuntu)**
  ```bash
  wget https://github.com/mh-dx/portier-cli/releases/latest/download/portier-cli_<VERSION>_linux_<ARCH>.deb
  ```
  ```bash
  sudo dpkg -i portier-cli_<VERSION>_linux_<ARCH>.deb
  ```

- **Red Hat-based systems (e.g., Fedora, CentOS)**
  ```bash
  wget https://github.com/mh-dx/portier-cli/releases/latest/download/portier-cli_<VERSION>_linux_<ARCH>.rpm
  ```
  ```bash
  sudo rpm -i portier-cli_<VERSION>_linux_<ARCH>.rpm
  ```

- **Tarball (any Linux):**
  ```bash
  wget https://github.com/mh-dx/portier-cli/releases/latest/download/portier-cli_<VERSION>_linux_<ARCH>.tar.gz
  ```
  ```bash
  tar -xzf portier-cli_<VERSION>_linux_<ARCH>.tar.gz
  ```
  ```bash
  sudo mv portier-cli /usr/local/bin
  ```

### Windows

For Windows, download the `.zip` file and extract it:

- **64-bit:**
  ```cmd
  curl -LO https://github.com/mh-dx/portier-cli/releases/latest/download/portier-cli_<VERSION>_windows_amd64.zip
  unzip portier-cli_<VERSION>_windows_amd64.zip -d portier-cli
  ```

- **ARM64:**
  ```cmd
  curl -LO https://github.com/mh-dx/portier-cli/releases/latest/download/portier-cli_<VERSION>_windows_arm64.zip
  unzip portier-cli_<VERSION>_windows_arm64.zip -d portier-cli
  ```

Add the folder to your PATH to run `portier-cli` from the command line.

## Building from Source

To build and install `portier-cli` from the source:

1. Clone the repository:
   ```bash
   git clone https://github.com/mh-dx/portier-cli.git
   cd portier-cli
   ```

2. Build the project using `make`:
   ```bash
   make
   ```

3. Install the binary:
   ```bash
   sudo make install
   ```

This will compile the source code and install the binary using `go install`.

## Verifying Installation

Verify that `portier-cli` is correctly installed by checking its version:
```bash
portier-cli version
```
# Setup

## Login

Login to portier.dev:
```
portier-cli login
```
This will display a one-time login link.
```
2024/05/04 20:11:26 Starting Portier CLI...
2024/05/04 20:11:26 

Logging in to portier.dev
-------------------------
Steps:

1. Open the following link in your browser to authenticate:
https://portier-spider.eu.auth0.com/activate?user_code=MXXG-ZXXG

2. Alternatively, open https://portier-spider.eu.auth0.com/activate in your browser and enter the code MXXG-ZXXG

Waiting for user to log in...
```
In some shells, you can click the link directly, on others you have to copy the link and open it in your browser. Complete the login in your browser. After a short while, portier-cli will also display a success message:

```
2024/05/04 20:11:23 Log in successful, storing access token in ~/.portier/credentials.yaml
2024/05/04 20:11:23 Login successful.
```

## Register a device

Now it's time to register this machine as a device:
```
portier-cli register --name myDevice1
```
This will connect to the portier API to register the device and download an API key:
```
2024/04/12 21:14:32 Starting Portier CLI...
Command: Create device myDevice1 at https://api.portier.dev/api, out yaml
Registering Device...
Generating API key...
Device registered and credentials stored successfully.
Device ID: 	cd9b0785-5f26-405f-beed-b2568a2d9efe
API Key: 	  ***
```

Take a note of your Device ID (in this case `cd9b0785-5f26-405f-beed-b2568a2d9efe`). We'll need it later when configuring a service.

## Start Portier

After you've registered, you can start the service:
```
portier-cli run
```
The output:
```
2024/04/12 21:18:40 Starting Portier CLI...
starting device, services /Users/mario/.portier/config.yaml, apiToken /Users/mario/.portier/credentials_device.yaml, out json
2024/04/12 21:18:40 Creating relay...
2024/04/12 21:18:40 Starting services...
2024/04/12 21:18:40 All Services started...
```

In this example, myDevice1 can be accessed remotely by other portier devices belonging to your account. Note that myDevice1 doesn't forward any remote port itself, it is just waiting for incoming connections. Read the next chapter to learn how you can setup a second portier device to access myDevice1.

## Setting Up a Remote Service

Assume you need to access myDevice1's via ssh, where the ssh server on myDevice1 is running on port 22. Let's call your home machine `myHome`.

First, repeat the previous steps to install and register portier-cli on your home machine. Then, create the portier config.yaml in your home folder, (under `~/.portier/config.yaml`), and add the following lines:
```
services:
  - name: ssh
    options: 
      urlLocal: "tcp://localhost:22222"                      # the local URL on myHome, where the remote port will be forwarded to
      urlRemote: "tcp://localhost:22"                        # the URL that portier-cli on myDevice1 will connect to
      peerDeviceID: <Device ID>                              # the device id we noted in chapter "Register a device"
      ReadBufferSize: 32768                                  # optional TCP read buffer size (set to 32KB to accomodate scp)
```

Now, start portier-cli on myHome, and note the additional output about the started service ssh:
```
% ./portier-cli run          
2024/04/13 21:04:53 Starting Portier CLI...
starting device, services /Users/mario/.portier/config.yaml, apiToken /Users/mario/.portier/credentials_device.yaml, out json
2024/04/13 21:04:53 Creating relay...
2024/04/13 21:04:53 Starting services...
2024/04/13 21:04:53 Starting service: ssh
2024/04/13 21:04:53 Starting listener for service: {ssh {tcp://localhost:22222 tcp://localhost:22 34d34526-9d00-4f17-90e9-5c87b8e01703      0s 0s 0 0}}
...
2024/04/13 21:04:53 All Services started...
```

Now, you're ready to access myDevice1 from myHome:
```
ssh -p 22222 root@localhost
```

Congratulations! You successfully forwarded a port using portier.

# End-to-End Encryption

portier connections can optionally be end-to-end encrypted using TLS 1.3. With encryption enabled, even simple plain-text protocols like http can only be read by the communicating devices. Not even portier.dev is able to decrypt the traffic. To use encryption, two simple steps are needed for each device taking part in an encrypted connection:

1. Creation of a TLS certificate and upload of its public fingerprint to portier.dev via the `portier-cli tls create` command
2. Download of the peer devices's fingerprint from portier.dev via the `portier-cli tls trust` command

## Creation

## Trusting a Peer Device

# Project Layout
* [assets/](https://pkg.go.dev/github.com/mh-dx/portier-cli/assets) => docs, images, etc
* [cmd/](https://pkg.go.dev/github.com/mh-dx/portier-cli/cmd)  => commandline configurartions (flags, subcommands)
* [pkg/](https://pkg.go.dev/github.com/mh-dx/portier-cli/pkg)  => packages that are okay to import for other projects
* [internal/](https://pkg.go.dev/github.com/mh-dx/portier-cli/pkg)  => packages that are only for project internal purposes
- [`tools/`](tools/) => for automatically shipping all required dependencies when running `go get` (or `make bootstrap`) such as `golang-ci-lint` (see: https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module)
)
- [`scripts/`](scripts/) => build scripts 

# Makefile Targets
```sh
$> make
bootstrap                      install build deps
build                          build golang binary
clean                          clean up environment
cover                          display test coverage
docker-build                   dockerize golang application
fmt                            format go files
help                           list makefile targets
install                        install golang binary
lint                           lint go files
pre-commit                     run pre-commit hooks
run                            run the app
test                           display test coverage
```

# Contribute
If you find issues in that setup or have some nice features / improvements, I would welcome an issue or a PR :)

# Environment Variables
| Name             | Value            |
|------------------|------------------|
|PORTIER_HOME      | ~/.portier       |
