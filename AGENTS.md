# AGENTS.md

**IMPORTANT**: All commit messages must follow semantic commit conventions. Agents should always use the subject "AGENT" in their commits.

## Repository Summary: Portier CLI

### Overview
Portier CLI is a sophisticated network tunneling and remote access tool written in Go that enables secure, reliable remote port forwarding between devices through a cloud-based relay infrastructure. The project provides a command-line interface for establishing encrypted tunnels and port forwarding without complex network configuration.

### Architecture & Core Components

#### 1. Command Structure (`cmd/`)
- **Root Command**: Main CLI entry point with subcommands
- **Core Commands**:
  - `login` - PKCE-based authentication flow requiring browser
  - `register` - Device registration and API key management
  - `run` - Main service execution for tunnel management
  - `version` - Version information
  - `man` - Manual page generation
  - **TLS Management** (`ptls/`):
    - `create` - TLS certificate generation and fingerprint upload
    - `trust` - Peer device certificate trust management

#### 2. Relay Infrastructure (`internal/portier/relay/`)
This is the heart of the tunneling system:

- **Uplink Layer** (`uplink/`): WebSocket-based connection to Portier cloud service
  - Automatic reconnection with exponential backoff
  - Message encoding/decoding with msgpack
  - Event-driven connection state management

- **Router Layer** (`router/`): Message routing and connection management
  - Dynamic connection creation for inbound/outbound tunnels
  - Connection lifecycle management
  - Message dispatching between adapters

- **Adapter Layer** (`adapter/`): Protocol adaptation and state management
  - **Connection States**: Connecting (inbound/outbound) → Connected → Closed
  - **Forwarder**: Handles bidirectional data flow with acknowledgments
  - **Window Management**: Sliding window protocol for reliable delivery
  - **Message Heap**: Out-of-order message buffering and reordering

#### 3. Application Layer (`internal/portier/application/`)
- Service orchestration and lifecycle management
- Local TCP/UDP listener management
- Integration between relay system and local services
- TLS encryption integration for end-to-end security

#### 4. Configuration System (`internal/portier/config/`)
- YAML-based configuration management
- Device credentials and API token handling
- Service definitions with peer device mapping
- TLS/PTLS configuration for encryption

#### 5. Encryption Layer (`internal/portier/ptls/`)
- Optional end-to-end TLS 1.3 encryption
- Certificate management and fingerprint verification
- Client/server TLS bridging for transparent encryption
- Known hosts management for peer verification

### Key Technical Features

#### Networking
- **Protocol Support**: TCP tunneling
- **Cloud Relay**: WebSocket-based connection to portier.dev infrastructure
- **NAT Traversal**: No complex network setup required
- **High Throughput**: Optimized for low-latency, high-bandwidth scenarios

#### Reliability
- **Automatic Reconnection**: Robust connection recovery mechanisms
- **Message Acknowledgment**: Reliable delivery with sequence numbers
- **Flow Control**: Sliding window protocol with configurable buffers
- **Error Handling**: Comprehensive error recovery and reporting

#### Security
- **Device Authentication**: API token-based device registration
- **End-to-End Encryption**: Optional TLS 1.3 encryption between peers
- **Certificate Management**: Automated certificate creation and trust establishment
- **Known Hosts**: SSH-style peer verification system

### Development & Build System

#### Technology Stack
- **CLI Framework**: Cobra for command structure
- **WebSocket**: Gorilla WebSocket for real-time communication
- **Serialization**: MessagePack for efficient binary encoding
- **Testing**: Comprehensive test suite with mocks and integration tests

#### Build & Release
- **GoReleaser**: Automated multi-platform builds (Linux, macOS, Windows)
- **Docker**: Container support with multi-arch images
- **Package Management**: Native packages for various distributions
- **Completions**: Shell completion scripts (bash, zsh, fish)

#### Quality Assurance
- **Linting**: golangci-lint with extensive ruleset
- **Testing**: Unit tests, integration tests, and stress tests
- **Coverage**: Codecov integration for test coverage tracking
- **CI/CD**: GitHub Actions for automated testing and releases

### Use Cases & Deployment

#### Primary Use Cases
1. **Remote SSH Access**: Secure shell access to remote machines
2. **Database Tunneling**: Access to remote databases through encrypted tunnels
3. **Development Services**: Remote access to development servers and APIs
4. **IoT Connectivity**: Connecting to devices behind NAT/firewalls

#### Deployment Modes
- **Standalone Binary**: Single executable for all platforms
- **Docker Container**: Containerized deployment with configuration volumes
- **Service Mode**: Background daemon for persistent tunneling
- **CLI Mode**: Interactive command-line usage

### Performance Characteristics
- **Memory Usage**: ~10MB RAM baseline, scales with connection count
- **Latency**: Millisecond-level latency for interactive protocols
- **Throughput**: Handles high-bandwidth transfers (tested with 10MB+ payloads)
- **Concurrency**: Supports thousands of parallel connections
- **Resilience**: Automatic recovery from network interruptions

This repository represents a production-ready networking tool with enterprise-grade reliability, security, and performance characteristics, suitable for both individual developers and large-scale deployments.
