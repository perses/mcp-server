# Installation

There are several ways of installing the Perses MCP Server:

- [Using the precompiled binaries](#precompiled-binaries)
- [Using a container image](#container-images)
- [Building it from source](#building-from-source)

Whatever the method you choose, the MCP server needs a running [Perses](https://perses.dev) instance and a
[configuration](./configuration.md) telling it how to reach Perses.

## Precompiled binaries

Download the precompiled binaries from the [GitHub releases page](https://github.com/perses/mcp-server/releases).
It is recommended to use the latest release available.

Archives are available for Linux, macOS and Windows, on the `x86_64`, `arm64` and `i386` architectures (`i386` is not
available for macOS). Each archive contains the binary `perses-mcp-server`.

For example, on macOS with an Apple silicon processor:

```bash
curl -LO "https://github.com/perses/mcp-server/releases/latest/download/perses-mcp-server_Darwin_arm64.tar.gz"
tar -xzf perses-mcp-server_Darwin_arm64.tar.gz
```

Then move the binary to a stable location (for example `/usr/local/bin`) and note its absolute path: you need it to
[configure your LLM host](./clients.md).

## Container images

Container images are available on [Docker Hub](https://hub.docker.com/r/persesdev/mcp-server) and on
[Quay](https://quay.io/repository/persesdev/mcp-server), for the `amd64` and `arm64` platforms.

### Streamable HTTP mode

To run the MCP server as an HTTP service, set the transport to `http` and publish the port it listens on (`8000` by
default):

```bash
docker run --name perses-mcp-server -d -p 127.0.0.1:8000:8000 \
  -e PERMCP_TRANSPORT=http \
  -e PERMCP_PERSES_SERVER_URL=https://perses.example.com \
  -e PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN=<LOGIN> \
  -e PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD=<PASSWORD> \
  persesdev/mcp-server
```

The MCP endpoint is then available at `http://localhost:8000/mcp`.

The MCP server is configured here with [environment variables](./configuration.md#environment-variables). You can also
mount a configuration file and pass it with the flag `--config`:

```bash
docker run --name perses-mcp-server -d -p 127.0.0.1:8000:8000 \
  -v "$(pwd)/config.yaml:/etc/perses-mcp-server/config.yaml:ro" \
  persesdev/mcp-server --config=/etc/perses-mcp-server/config.yaml
```

Before exposing the MCP server beyond your machine, read the [security considerations](./deployment.md#security).

### stdio mode

The container image can also be started by the LLM host in stdio mode. In this case, the option `-i` is required to keep
the standard input open:

```bash
docker run -i --rm \
  -e PERMCP_PERSES_SERVER_URL=https://perses.example.com \
  -e PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN=<LOGIN> \
  -e PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD=<PASSWORD> \
  persesdev/mcp-server
```

See [MCP clients](./clients.md#using-the-container-image) to use this command in your LLM host.

> [!NOTE]
> Inside a container, `localhost` refers to the container itself. If Perses runs on your machine, reach it with
> `http://host.docker.internal:8080` (Docker Desktop), `http://host.containers.internal:8080` (Podman) or with the
> option `--network host` (Linux).

### Tags

We are publishing a set of tags to match different requirements:

1. There are two types of image: `distroless` and `distroless-debug`.
   The first one is meant to be used in production as it doesn't contain anything except the MCP server binary.
   The second one contains a shell and should be used to debug any issue you might encounter with your container.
2. Each release is available with the full version (`v0.2.0`) or only the minor version (`v0.2`), with or without the
   distribution suffix (`v0.2.0-distroless`, `v0.2.0-distroless-debug`). Without suffix, the image is the `distroless`
   one, and the suffix `-debug` is an alias for `-distroless-debug`.
3. The tag `latest` is an alias for the `distroless` distribution of the latest release.
4. The tag `latest-debug` is an alias for the `distroless-debug` distribution of the latest release.

Examples:

```bash
docker pull persesdev/mcp-server:v0.2.0
docker pull persesdev/mcp-server:v0.2-debug
docker pull quay.io/persesdev/mcp-server:v0.2.0-distroless
```

## Building from source

To build the MCP server from source, you need [Go](https://go.dev/doc/install) in the version defined in the
[`go.mod`](https://github.com/perses/mcp-server/blob/main/go.mod) file of the repository.

Start by cloning the repository:

```bash
git clone https://github.com/perses/mcp-server.git
cd mcp-server
```

Then build the MCP server with:

```bash
make build
```

The binary is created in the folder `bin`: `bin/perses-mcp-server`.

## What's next

- [Connect the MCP server to your LLM host](./clients.md)
- [Choose how to deploy it](./deployment.md)
- [Configure it](./configuration.md)
