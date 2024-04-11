# portier-cli

<div align="center">
Remotely access all your machines through Portier CLI. It's easy, efficient and reliable. For more info, visit www.portier.dev !
<br>
<br>

## Forget networking, we love the web.

If complex network setup blocked you - search no more. Portier offers you remote connectivity with literally zero network setup. Use your machine from home, no matter how it accesses the public internet. Web-access to portier.dev (HTTP and Websockets) is the only requirement to use our services.

## Robust, reliable and lean.

With its automatic reconnect and advanced retransmission algorithms, your remote access works free from connection drops. Portier turns these things into a bit of latency, and then everything continues smoothly.
Portier-cli, our client application written in golang, requires roughly 10MB of RAM to run, but is also capable of scaling up to handle thousands of parallel connections - if you need it.

## Secure, (don’t) trust us.

Portier uses TLS to secure your connections. And there’s no need to trust us: Portier-cli encrypts connections end to end (under development). Your data remains private.

## Low-latency, high throughput servers.

Portier uses a cloud infrastructure to forward messages between clients that handles high throughput with millisecond latencies. Working with rdp or ssh? Don’t worry about it, your clicks and key strokes will have a swift and fast response, just like you’re used to.

<br>
<br>
<img src="https://github.com/marinator86/portier-cli/actions/workflows/test.yml/badge.svg" alt="drawing"/>
<img src="https://pkg.go.dev/badge/github.com/marinator86/portier-cli.svg" alt="drawing"/>
<img src="https://img.shields.io/github/v/release/marinator86/portier-cli" alt="drawing"/>
<img src="https://img.shields.io/docker/pulls/marinator86/portier-cli" alt="drawing"/>
<img src="https://img.shields.io/github/downloads/marinator86/portier-cli/total.svg" alt="drawing"/>
</div>

# Table of Contents
<!--ts-->
   * [portier-cli](#portier-cli)
   * [Project Layout](#project-layout)
   * [Makefile Targets](#makefile-targets)
   * [Contribute](#contribute)

# Project Layout
* [assets/](https://pkg.go.dev/github.com/marinator86/portier-cli/assets) => docs, images, etc
* [cmd/](https://pkg.go.dev/github.com/marinator86/portier-cli/cmd)  => commandline configurartions (flags, subcommands)
* [pkg/](https://pkg.go.dev/github.com/marinator86/portier-cli/pkg)  => packages that are okay to import for other projects
* [internal/](https://pkg.go.dev/github.com/marinator86/portier-cli/pkg)  => packages that are only for project internal purposes
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
