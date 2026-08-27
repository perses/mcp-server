# Running the Remote Perses MCP Server against an OIDC-protected Perses (Client-Credentials / Technical User)

This one-page guide shows how to host the **Perses MCP Server as a remote server**
(Streamable HTTP transport) against a **Perses** instance protected by an **OIDC
provider**, authenticating as a **technical (machine) user** with the **OAuth 2.0
client-credentials flow**. MCP hosts (OpenCode, Claude Desktop, VS Code, Cursor, …)
connect to it over HTTP by URL — they never see any credentials.

Keycloak is used as the example provider, but the setup is generic: any OIDC provider
that supports the client-credentials grant (Okta, Auth0, Entra ID, Ping, Dex, …) works the
same way — only the provider-side client configuration and issuer URL change.

---

## What you're setting up

The MCP server authenticates to Perses using the OIDC client-credentials flow. Two things
about this are worth knowing before you edit any config, because they determine values you
have to fill in:

- **Perses brokers the flow and issues its own token.** The MCP server sends its
  `client_id`/`client_secret` to **Perses's** own token endpoint (not the OIDC provider's):

  ```bash
  curl -s -u "$CLIENT_ID:$CLIENT_SECRET" \
    -d 'grant_type=client_credentials' \
    https://<perses-host>/api/auth/providers/oidc/<slug_id>/token
  ```

  where `<slug_id>` is the identifier you assign to the OIDC provider in the Perses config
  (`oidc.slug_id`, e.g. `keycloak`). Perses validates the credentials with your OIDC
  provider and returns a Perses-native token. So the `tokenURL` you configure in the MCP
  server points at **Perses**, not at the provider. (Perses finds the provider's real token
  endpoint via the `issuer` you give it — see Step 2.)
- **The user's username equals the `client_id`.** Perses creates a user whose name is the
  `client_id`; that's the name you bind to a role in Step 2.

Access is then controlled entirely by **Perses RBAC**: grant the technical user only the
permissions it needs (e.g. read-only). Disallowed actions return `403`, even if issued
directly against the API. No MCP server code changes are needed — this is pure
configuration via the `perses_server.oauth` block.

---

## Step 1 — Configure the OIDC provider

Create a **confidential client** for the MCP server and enable the client-credentials
grant:

| Setting                       | Value                                                                 |
| ----------------------------- | --------------------------------------------------------------------- |
| Client type                   | Confidential (has a client secret)                                    |
| Client authentication         | Enabled (`client-secret`)                                             |
| Client-credentials grant      | Enabled — in Keycloak this is **Service accounts roles**              |
| Interactive flows             | Disabled (standard/implicit/direct-access not needed for a machine)   |

Note the **`client_id`**, the generated **`client_secret`**, and the provider's **issuer
URL** (Keycloak: `https://<host>/realms/<realm>`). Verify the grant works directly:

```bash
curl -s -u "$CLIENT_ID:$CLIENT_SECRET" \
  -d 'grant_type=client_credentials' \
  https://<oidc-host>/realms/<realm>/protocol/openid-connect/token
# => a JSON object containing "access_token" and "token_type": "Bearer"
```

> For other providers, use their token endpoint from the discovery document
> (`{issuer}/.well-known/openid-configuration` → `token_endpoint`).

---

## Step 2 — Configure Perses

Enable auth + RBAC and register your OIDC provider in the Perses server config:

```yaml
security:
  enable_auth: true
  encryption_key: "<exactly-32-bytes>"          # AES-256 key to encrypt secrets at rest
  authentication:
    providers:
      enable_native: false
      oidc:
        - slug_id: keycloak                       # appears in the /token URL path
          name: Keycloak
          client_id: perses-mcp
          client_secret: perses-mcp-secret
          issuer: https://<oidc-host>/realms/<realm>
          scopes: [openid, profile, email]
  authorization:
    provider:
      native:
        guest_permissions: []                     # no default access -> RBAC decides
```

- **`slug_id`** is used later in the MCP server's `tokenURL`.
- **`issuer`** must be reachable from the Perses process and match the token issuer.
- **`guest_permissions: []`** means an authenticated user has *no* access until a role
  binding grants it — the foundation of a least-privilege technical user.

### Grant the technical user read-only access (RBAC)

The synced user's **username equals the OIDC `client_id`**. Bind it to a read-only role.
Provision these resources (via Perses provisioning folder or the API):

```yaml
# read-only GlobalRole
kind: GlobalRole
metadata:
  name: readonly
spec:
  permissions:
    - actions: [read]
      scopes: ["*"]
---
# bind the technical user (username == client_id) to the read-only role
kind: GlobalRoleBinding
metadata:
  name: mcp-readonly
spec:
  role: readonly
  subjects:
    - kind: User
      name: perses-mcp
```

> Scope RBAC as narrowly as your use case allows (specific projects/resources instead of
> `read` on `*`).

---

## Step 3 — Configure and run the remote MCP server

Configure the server as a **remote** server (HTTP transport) and point it at Perses with
the `oauth` block. The `tokenURL` targets **Perses's own `/token` endpoint** (not the OIDC
provider's):

```yaml
transport: http            # remote server (Streamable HTTP)
listen_address: ":8000"    # endpoint served at http://<mcp-host>:8000/mcp

perses_server:
  url: "https://<perses-host>"
  oauth:
    clientID: "perses-mcp"
    clientSecret: "perses-mcp-secret"
    # Perses brokers with the OIDC provider. slug_id ("keycloak") must match Step 2.
    tokenURL: "https://<perses-host>/api/auth/providers/oidc/keycloak/token"
    authStyle: 2      # 2 = send client_id/secret via HTTP Basic (required by Perses)
```

### Keep the credentials out of the config file

When the server is hosted, don't commit the `client_id`/`client_secret` to the YAML file.
The MCP server resolves config values from environment variables with the `PERMCP_`
prefix (each key uppercased, nested keys joined with `_`), and **environment variables
override the file**. So you can leave the `oauth` block in the file but omit the secret
values and supply them from your secret manager / orchestrator instead:

| Environment variable                          | Config path                          |
| --------------------------------------------- | ------------------------------------ |
| `PERMCP_PERSES_SERVER_OAUTH_CLIENTID`         | `perses_server.oauth.clientID`       |
| `PERMCP_PERSES_SERVER_OAUTH_CLIENTSECRET`     | `perses_server.oauth.clientSecret`   |
| `PERMCP_PERSES_SERVER_OAUTH_TOKENURL`         | `perses_server.oauth.tokenURL`       |
| `PERMCP_PERSES_SERVER_OAUTH_AUTHSTYLE`        | `perses_server.oauth.authStyle`      |

Example (e.g. in a systemd unit, container env, or Kubernetes Secret):

```bash
export PERMCP_PERSES_SERVER_OAUTH_CLIENTID="perses-mcp"
export PERMCP_PERSES_SERVER_OAUTH_CLIENTSECRET="perses-mcp-secret"
```

> Prefer a file-based secret? Use `clientSecretFile` in the YAML (env var
> `PERMCP_PERSES_SERVER_OAUTH_CLIENTSECRETFILE`) and mount the secret as a file. Configure
> **either** `clientSecret` **or** `clientSecretFile`, not both.

Start it:

```bash
perses-mcp-server --config permcp.yaml
```

You should see `transport=http`, `Tools registered successfully`, and the server
listening on `listen_address`.

> **Env-var precedence gotcha:** because env vars override the file, leftover vars from
> other setups can silently change behavior. In particular, if
> `PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN` / `..._PASSWORD` are exported, the server sees
> *two* auth methods and fails with `only one type of authentication should be configured`.
> `unset` them before launching.

---

## Step 4 — Connect your MCP host

Point your MCP host at the server's HTTP endpoint. Example OpenCode config
(`~/.config/opencode/opencode.json` or a project `opencode.json`):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "perses-remote": {
        "type": "remote",
        "url": "http://<mcp-host>:8000/mcp",
        "oauth": false
      }
    }
  }
}
```

`oauth: false` because the MCP endpoint itself is not OAuth-protected — auth to Perses
happens *inside* the server via its config. Then ask natural-language questions
("List all projects in Perses"); write attempts return `403`, proving the technical user
is read-only.

---

## Production notes

- **Never hardcode secrets.** Store the `client_secret` outside the config file — either
  via the `PERMCP_PERSES_SERVER_OAUTH_CLIENTSECRET` env var or
  `perses_server.oauth.clientSecretFile` — and source it from a secret manager (see
  Step 3).
- **Use TLS everywhere** (OIDC provider, Perses, and the MCP endpoint) with correct
  issuer/hostname values.
- **Give the technical user its own OIDC client**, separate from Perses's own relying-party
  client, so its identity and permissions are isolated. Perses supports a
  `client_credentials` override block on the OIDC provider for this.
- **Scope RBAC narrowly** to only the projects/resources the MCP server needs.
- **Rotate** the client secret regularly; the flow's automatic token refresh handles new
  tokens transparently.
