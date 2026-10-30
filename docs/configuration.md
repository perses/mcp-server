# Configuration

The Perses MCP Server is configured with command-line flags, a configuration file and environment variables.

## Flags

```bash
  -config string
    	Path to the YAML configuration file
  -log.format string
    	log format. Possible value: text, json (default "text")
  -log.level string
    	log level. Possible value: panic, fatal, error, warning, info, debug, trace (default "info")
  -log.method-trace
    	include the calling method as a field in the log. Can be useful to see immediately where the log comes from
```

Example:

```bash
perses-mcp-server --config=./config.yaml --log.level=debug
```

The logs are written on the standard error output, so they never interfere with the MCP messages exchanged on the
standard output in stdio mode.

## Configuration file

The MCP server can be configured using a YAML file and/or environment variables. The values are resolved in this order,
the last one winning:

1. the default values,
2. the configuration file, passed with the flag `--config`,
3. the environment variables.

The configuration file is optional: the MCP server can be configured with environment variables only.

The configuration file is parsed strictly: the MCP server refuses to start when the file contains an unknown key, which
helps catching typos. Note that the keys of the sections shared with Perses (`authorization`, `oauth`, `basic_auth`,
`tls_config`...) are written in camelCase, like `credentialsFile` or `insecureSkipVerify`.

### Environment variables

To get the environment variable corresponding to a setting, join all the keys leading to it in the YAML with `_`, put
the result in uppercase and prefix it with `PERMCP_`. For example, `perses_server.native_auth.password` becomes
`PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD`, and `perses_server.oauth.clientSecret` becomes
`PERMCP_PERSES_SERVER_OAUTH_CLIENTSECRET`.

It is particularly useful for the sensitive values, like the passwords and the tokens, that should not be stored in the
configuration file.

| Environment variable                                 | Configuration path                           |
|------------------------------------------------------|----------------------------------------------|
| `PERMCP_TRANSPORT`                                   | `transport`                                  |
| `PERMCP_LISTEN_ADDRESS`                              | `listen_address`                             |
| `PERMCP_READ_ONLY`                                   | `read_only`                                  |
| `PERMCP_RESOURCES`                                   | `resources`                                  |
| `PERMCP_PERSES_SERVER_URL`                           | `perses_server.url`                          |
| `PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN`             | `perses_server.native_auth.login`            |
| `PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD`          | `perses_server.native_auth.password`         |
| `PERMCP_PERSES_SERVER_AUTHORIZATION_TYPE`            | `perses_server.authorization.type`           |
| `PERMCP_PERSES_SERVER_AUTHORIZATION_CREDENTIALS`     | `perses_server.authorization.credentials`    |
| `PERMCP_PERSES_SERVER_AUTHORIZATION_CREDENTIALSFILE` | `perses_server.authorization.credentialsFile` |
| `PERMCP_PERSES_SERVER_OAUTH_CLIENTID`                | `perses_server.oauth.clientID`               |
| `PERMCP_PERSES_SERVER_OAUTH_CLIENTSECRET`            | `perses_server.oauth.clientSecret`           |
| `PERMCP_PERSES_SERVER_OAUTH_CLIENTSECRETFILE`        | `perses_server.oauth.clientSecretFile`       |
| `PERMCP_PERSES_SERVER_OAUTH_TOKENURL`                | `perses_server.oauth.tokenURL`               |
| `PERMCP_PERSES_SERVER_OAUTH_AUTHSTYLE`               | `perses_server.oauth.authStyle`              |
| `PERMCP_PERSES_SERVER_K8S_AUTH_KUBECONFIG`           | `perses_server.k8s_auth.kubeconfig`          |
| `PERMCP_PERSES_SERVER_TLS_CONFIG_CAFILE`             | `perses_server.tls_config.caFile`            |
| `PERMCP_PERSES_SERVER_TLS_CONFIG_INSECURESKIPVERIFY` | `perses_server.tls_config.insecureSkipVerify` |

> [!WARNING]
> As the environment variables override the configuration file, a variable left over from another setup can silently
> change the behavior of the MCP server. For example, if `PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN` is set while the
> configuration file uses `oauth`, the MCP server sees two authentication methods and refuses to start with the error
> `only one type of authentication should be configured`.

The MCP server also honors the standard environment variables `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` to reach
Perses.

### Definition

The file is written in YAML format, defined by the scheme described below. Brackets indicate that a parameter is
optional.

Generic placeholders are defined as follows:

- `<boolean>`: a boolean that can take the values `true` or `false`
- `<filename>`: a valid path to a file
- `<int>`: an integer value
- `<secret>`: a regular string that is a secret, such as a password
- `<string>`: a regular string
- `<url>`: a valid URL

```yaml
# The transport used to communicate with the MCP clients:
#   - stdio: the MCP server is started by the LLM host, and communicates through the standard input and output.
#   - http: the MCP server listens for Streamable HTTP requests on listen_address.
# "streamable-http" and "http-streamable" are accepted as aliases of "http".
[ transport: <string> | default = "stdio" ]

# The address to listen on when the transport is http. A port alone (e.g. "8000") is also accepted.
[ listen_address: <string> | default = ":8000" ]

# When true, only the tools reading data are registered.
# The tools creating, updating or deleting resources are not available to the LLM.
[ read_only: <boolean> | default = false ]

# A comma-separated list of resources. Only the tools of these resources are registered.
# When empty, the tools of all the resources are registered. See the list of the resources below.
[ resources: <string> ]

# How to reach Perses, and how to authenticate to it.
[ perses_server: <Perses_Server config> ]
```

Example:

```yaml
transport: stdio
read_only: true
resources: "project,dashboard,datasource,globaldatasource"

perses_server:
  url: "https://perses.example.com"
  native_auth:
    login: "mcp"
    # Better passed with the environment variable PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD
    password: "<PASSWORD>"
```

### Resources

The setting `resources` accepts the following names. They are case-insensitive, and the spaces around the commas are
ignored. The MCP server refuses to start if a name is unknown.

| Resource             | Tools                                                                 |
|----------------------|-----------------------------------------------------------------------|
| `project`            | [Projects](./tools.md#projects)                                       |
| `dashboard`          | [Dashboards](./tools.md#dashboards)                                   |
| `ephemeraldashboard` | [Ephemeral dashboards](./tools.md#ephemeral-dashboards)               |
| `datasource`         | [Project datasources](./tools.md#datasources)                         |
| `globaldatasource`   | [Global datasources](./tools.md#datasources)                          |
| `variable`           | [Project variables](./tools.md#variables)                             |
| `globalvariable`     | [Global variables](./tools.md#variables)                              |
| `role`               | [Project roles](./tools.md#roles-and-role-bindings)                   |
| `globalrole`         | [Global roles](./tools.md#roles-and-role-bindings)                    |
| `rolebinding`        | [Project role bindings](./tools.md#roles-and-role-bindings)           |
| `globalrolebinding`  | [Global role bindings](./tools.md#roles-and-role-bindings)            |
| `plugin`             | [Plugins](./tools.md#plugins)                                         |
| `validate`           | [Validation](./tools.md#validation)                                   |
| `migrate`            | [Grafana dashboards migration](./tools.md#grafana-dashboards-migration) |

> [!TIP]
> Each tool adds its description to the context of the LLM. Registering only the resources you need saves tokens and
> helps the LLM to pick the right tool.

### Perses_Server Config

```yaml
# The URL of Perses.
[ url: <url> | default = "http://localhost:8080" ]

# The authentication to Perses. At most one of the following methods can be configured.
# No authentication is needed when the authentication is disabled in Perses.
[ native_auth: <NativeAuth config> ]
[ authorization: <Authorization config> ]
[ oauth: <OAuth config> ]
[ k8s_auth: <K8sAuth config> ]
[ basic_auth: <BasicAuth config> ]

# The TLS configuration used to connect to Perses.
[ tls_config: <TLS config> ]

# Additional HTTP headers sent with every request to Perses.
[ headers:
    [ <string>: <string> ... ] ]
```

#### Choosing an authentication method

The authentication methods depend on how
[the authentication is configured in Perses](https://perses.dev/docs/perses/latest/concepts/authentication/):

| Method                                    | When to use it                                                                                       | Token renewal                                                                       |
|-------------------------------------------|------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------|
| [`native_auth`](#nativeauth-config)       | You have a user of the native provider of Perses (login and password).                                | Automatic: the MCP server logs in, then refreshes its session when needed.          |
| [`oauth`](#oauth-config)                  | Perses uses an OIDC or OAuth provider, and you have a client allowed to use the client credentials grant. | Automatic: a new token is requested when the previous one expires.                  |
| [`k8s_auth`](#k8sauth-config)             | Perses uses the Kubernetes authentication.                                                            | None: the token is read once, when the MCP server starts.                           |
| [`authorization`](#authorization-config)  | You already have a token, for example the one returned by `percli whoami --show-token`.              | None: a Perses token expires after the `access_token_ttl` of Perses (15 min by default). |
| [`basic_auth`](#basicauth-config)         | Perses is behind a proxy requiring an HTTP Basic authentication.                                     | Not applicable.                                                                      |

For a long-running MCP server (see [Deployment](./deployment.md#shared-deployment-with-streamable-http)), prefer a
method with an automatic renewal.

### NativeAuth Config

The credentials of a user of the
[native provider](https://perses.dev/docs/perses/latest/concepts/authentication/#native-provider) of Perses. The MCP
server logs in with them, and refreshes its session when it expires.

```yaml
login: <string>
password: <secret>
```

Example:

```yaml
perses_server:
  url: "https://perses.example.com"
  native_auth:
    login: "mcp"
    password: "<PASSWORD>"
```

### Authorization Config

A static token sent in the HTTP header `Authorization` of each request.

```yaml
# The type of the authorization.
[ type: <string> | default = "Bearer" ]

# The credentials, like a token.
# At most one of credentials and credentialsFile is allowed.
[ credentials: <secret> ]
[ credentialsFile: <filename> ]
```

To get a Perses token, log in with [percli](https://perses.dev/docs/perses/latest/cli/), then print the token:

```bash
percli login https://perses.example.com
percli whoami --show-token
```

Example:

```yaml
perses_server:
  url: "https://perses.example.com"
  authorization:
    credentialsFile: "/path/to/token"
```

> [!WARNING]
> The Perses tokens expire after the `access_token_ttl` set in the
> [Perses configuration](https://perses.dev/docs/perses/latest/configuration/configuration/) (15 minutes by default).
> The MCP server can't renew them: the requests fail once the token has expired.

### OAuth Config

The [OAuth 2.0 client credentials flow](https://datatracker.ietf.org/doc/html/rfc6749#section-4.4), meant for
technical accounts. The MCP server requests a token with its client ID and secret, and requests a new one when it
expires.

Perses brokers this flow: `tokenURL` must target the token endpoint **of Perses**, not the one of your identity
provider: `<PERSES_URL>/api/auth/providers/<oidc|oauth>/<SLUG_ID>/token`, where `<SLUG_ID>` is the `slug_id` of the
provider in the [Perses configuration](https://perses.dev/docs/perses/latest/configuration/configuration/#authentication-config).
Perses validates the credentials with the identity provider, and returns a Perses token. In Perses, the user created for
this technical account is named after the client ID: bind it to a [role](https://perses.dev/docs/perses/latest/concepts/authorization/)
to grant it permissions.

```yaml
# The client ID of the application.
clientID: <string>

# The client secret of the application.
# Exactly one of clientSecret and clientSecretFile is required.
[ clientSecret: <secret> ]
[ clientSecretFile: <filename> ]

# The token endpoint. See above.
tokenURL: <url>

# The scopes to request.
[ scopes:
    - <string> ... ]

# Additional parameters sent to the token endpoint.
[ endpointParams:
    [ <string>: [ <string> ... ] ... ] ]

# How the client ID and the client secret are sent to the token endpoint.
# 0: auto-detect, 1: in the parameters, 2: in the header (HTTP Basic authentication).
# Perses expects them in the header: use 2.
[ authStyle: <int> | default = 0 ]
```

Example:

```yaml
perses_server:
  url: "https://perses.example.com"
  oauth:
    clientID: "perses-mcp"
    clientSecretFile: "/etc/perses-mcp-server/client-secret"
    tokenURL: "https://perses.example.com/api/auth/providers/oidc/keycloak/token"
    authStyle: 2
```

### K8sAuth Config

The token of a Kubernetes service account or user, when Perses is configured with the
[Kubernetes authentication](https://perses.dev/docs/perses/latest/concepts/authorization/#kubernetes).

```yaml
# The path to a kubeconfig file. The token of its current context is used.
# When not set, the MCP server uses the token of the service account of the pod it is running in.
[ kubeconfig: <filename> ]
```

The token is read once, when the MCP server starts. It must be available in the kubeconfig file (fields `token` or
`tokenFile` of the user): the credential plugins (`exec`, `auth-provider`) are not supported.

Example, for an MCP server running in the same cluster as Perses:

```yaml
perses_server:
  url: "http://perses.perses.svc:8080"
  k8s_auth: {}
```

### BasicAuth Config

The credentials sent in an HTTP Basic `Authorization` header. Perses itself doesn't accept them: use this method only
when a proxy in front of Perses requires an HTTP Basic authentication. To log in with a Perses user, use
[`native_auth`](#nativeauth-config).

```yaml
username: <string>

# Exactly one of password and passwordFile is required.
[ password: <secret> ]
[ passwordFile: <filename> ]
```

### TLS Config

```yaml
# The CA certificate to validate the certificate of Perses with. At most one of ca and caFile is allowed.
[ ca: <string> ]
[ caFile: <filename> ]

# The certificate and the key for the client certificate authentication to Perses.
# At most one of cert and certFile is allowed.
# At most one of key and keyFile is allowed.
[ cert: <string> ]
[ certFile: <filename> ]
[ key: <secret> ]
[ keyFile: <filename> ]

# ServerName extension to indicate the name of the server.
# https://tools.ietf.org/html/rfc4366#section-3.1
[ serverName: <string> ]

# Disable the validation of the certificate of Perses.
[ insecureSkipVerify: <boolean> | default = false ]

# The minimum acceptable TLS version. Accepted values: TLS10 (TLS 1.0), TLS11 (TLS 1.1), TLS12 (TLS 1.2), TLS13 (TLS 1.3).
[ minVersion: <string> | default = "TLS12" ]

# The maximum acceptable TLS version. Accepted values: TLS10 (TLS 1.0), TLS11 (TLS 1.1), TLS12 (TLS 1.2), TLS13 (TLS 1.3).
[ maxVersion: <string> | default = "TLS13" ]
```

Example:

```yaml
perses_server:
  url: "https://perses.example.com"
  tls_config:
    caFile: "/etc/perses-mcp-server/ca.pem"
```
