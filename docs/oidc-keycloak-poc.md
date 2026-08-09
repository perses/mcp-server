# POC: Connecting the Perses **Remote** MCP Server to Perses with OIDC (Keycloak, Client-Credentials Flow)

This guide walks through a proof-of-concept that runs the **Perses MCP Server as a
*remote* server** (HTTP / Streamable HTTP transport) and connects it to a **Perses**
instance protected by an **OIDC provider (Keycloak)**, using the **OAuth 2.0
client-credentials flow**. The MCP server authenticates as a **technical (machine) user**
that has **read-only** access to Perses, enforced by Perses RBAC. An MCP host such as
**OpenCode** connects to the server over HTTP by URL.

All supporting files live next to this guide in [`oidc-keycloak-poc/`](./oidc-keycloak-poc/):

```
docs/oidc-keycloak-poc/
├── docker-compose.yaml                 # Keycloak + Perses
├── permcp-http.yaml                    # Remote MCP server config (HTTP transport + client-credentials)
├── keycloak/
│   └── realm-export.json               # realm "perses" + confidential client "perses-mcp"
└── perses/
    ├── config.yaml                     # Perses config: auth enabled + Keycloak OIDC + RBAC
    └── provisioning/
        ├── globalrole-readonly.yaml               # read-only GlobalRole
        ├── globalrolebinding-mcp-readonly.yaml    # binds user "perses-mcp" -> read-only
        ├── project-*.yaml                         # sample projects to read
        └── dashboard-*.yaml                       # sample dashboards to read
```

---

## 1. How it works (architecture)

The most important thing to understand: **Perses issues its own tokens.** It does not
accept a raw Keycloak access token as a bearer credential. For a machine client, Perses
exposes a dedicated *client-credentials* endpoint that **brokers** with the IdP:

```
POST {perses}/api/auth/providers/oidc/{slug_id}/token   (grant_type=client_credentials)
```

Flow:

```
                     ┌───────────────────────────┐
   MCP host          │   Remote MCP Server        │
 (OpenCode) ──HTTP──►│   (host, :8000, transport  │
  tool calls         │    = http)                 │
                     └──────────────┬─────────────┘
                                    │  client_id + client_secret (HTTP Basic)
                                    ▼
                            ┌────────────────┐   validates client_id/secret
                            │    Perses      │   (client-credentials grant)
                            │  (container)   │ ─────────────────────────────► ┌───────────┐
                            │                │ ◄─────────────────────────────  │ Keycloak  │
                            └───────┬────────┘        (brokered)               │(container)│
                                    │  Perses-native JWT                        └───────────┘
                                    ▼
                        MCP server calls the Perses API with
                        Authorization: Bearer <perses jwt>
```

1. OpenCode (or any MCP host) sends tool calls to the remote MCP server over HTTP.
2. The MCP server sends its **Keycloak `client_id` + `client_secret`** (HTTP Basic) to
   Perses's `/token` endpoint.
3. Perses forwards those credentials to **Keycloak** to validate them.
4. On success, Perses **syncs a user whose username = the `client_id`** (this is the
   "technical user") and returns a **Perses-native JWT**.
5. The MCP server sends that JWT as `Authorization: Bearer …` on every Perses API call.
   The token auto-refreshes when it expires (the flow simply re-runs).

The MCP server needs **no code changes** — this is pure configuration using the existing
`perses_server.oauth` block, which maps to the Perses client library's
`clientcredentials.Config`. (This is exactly what Perses's own e2e test
`TestAuth_OIDCProvider_Token_WithLib` exercises.)

> **Separation of concerns:** the LLM/MCP host only decides *which tool to call*. All
> authentication (OIDC client-credentials) happens **inside the MCP server** via its
> config. OpenCode only needs the server's **URL** — it never sees the client secret.

**Read-only enforcement** is done with **Perses RBAC**: the config grants *no* default
(guest) permissions, and a `GlobalRoleBinding` grants the `perses-mcp` user a read-only
`GlobalRole`. Any write attempt returns `403`, even if issued directly against the API.

---

## 2. Prerequisites

- Docker + Docker Compose
- Go (to build the MCP server) — `go 1.25+`
- `jq` (for the test commands below; optional)
- An MCP host to connect as a remote client (this guide uses **OpenCode**)
- The MCP server binary built from this repo:

  ```bash
  cd mcp-server
  make build            # produces bin/perses-mcp-server
  ```

- Free host ports: **8080** (Keycloak), **8082** (Perses), **8000** (remote MCP server).

> All commands below are run from `mcp-server/docs/oidc-keycloak-poc/` unless noted.

---

## 3. Step 1 — Set up Keycloak

This POC provisions Keycloak automatically via realm import, so you don't have to click
through the console. The realm file is
[`keycloak/realm-export.json`](./oidc-keycloak-poc/keycloak/realm-export.json) and defines:

- **Realm:** `perses`
- **Client:** `perses-mcp`
  - Confidential (`"publicClient": false`, `clientAuthenticatorType: client-secret`)
  - **Service accounts enabled** (`"serviceAccountsEnabled": true`) → this is what
    enables the **client-credentials** grant
  - Secret: `perses-mcp-secret` *(hardcoded for the POC — do not do this in production)*
  - Standard/implicit/direct-access flows disabled (not needed for a machine client)

Start Keycloak:

```bash
docker compose up -d keycloak
```

Wait until the realm's discovery document is available (takes ~30–40s on first boot):

```bash
until curl -sf http://localhost:8080/realms/perses/.well-known/openid-configuration >/dev/null; do
  echo "waiting for keycloak..."; sleep 3
done
echo "keycloak ready"
```

Sanity-check that the client-credentials grant works directly against Keycloak:

```bash
curl -s -u perses-mcp:perses-mcp-secret \
  -d 'grant_type=client_credentials' \
  http://localhost:8080/realms/perses/protocol/openid-connect/token | jq '{token_type, has_token: (.access_token != null)}'
# => { "token_type": "Bearer", "has_token": true }
```

<details>
<summary><b>Alternative: create the realm/client manually in the Admin Console</b></summary>

1. Open <http://localhost:8080> and log in with `admin` / `admin`
   (the `KC_BOOTSTRAP_ADMIN_*` values from `docker-compose.yaml`).
2. **Create realm** → name `perses`.
3. **Clients → Create client**:
   - Client type: **OpenID Connect**, Client ID: `perses-mcp`.
   - **Capability config:** turn **Client authentication** ON, enable
     **Service accounts roles** (this is the client-credentials grant). You can turn
     off Standard flow / Direct access grants.
4. **Clients → perses-mcp → Credentials**: copy the generated **Client secret** (or set
   it to `perses-mcp-secret` to match this guide). Update the secret everywhere below if
   you use a different value.

</details>

---

## 4. Step 2 — Configure the Perses application

Perses is configured by [`perses/config.yaml`](./oidc-keycloak-poc/perses/config.yaml).
Key parts:

```yaml
security:
  enable_auth: true
  encryption_key: "=tW$56zytgB&3jN2E%7-+qrGZE?v6LCc"   # exactly 32 bytes
  authentication:
    providers:
      enable_native: false
      oidc:
        - slug_id: keycloak                              # used in the /token URL path
          name: Keycloak
          client_id: perses-mcp
          client_secret: perses-mcp-secret
          issuer: http://keycloak:8080/realms/perses     # reachable from the Perses container
          scopes: [openid, profile, email]
  authorization:
    provider:
      native:
        guest_permissions: []                            # no default access -> RBAC decides
```

Notes:

- **`issuer`** uses the compose service name `keycloak`, not `localhost`, because
  **Perses (a container) resolves it over the compose network**. `KC_HOSTNAME_STRICT=false`
  makes Keycloak return an issuer matching the request host, so discovery succeeds.
- **`guest_permissions: []`** means an authenticated user has *no* permissions until a
  role binding grants them — this is the foundation of the read-only technical user.
- Because the MCP server authenticates with the same `perses-mcp` client that Perses uses
  as its relying party, no separate `client_credentials` override block is needed.

### Read-only RBAC (the technical user)

The [`perses/provisioning/`](./oidc-keycloak-poc/perses/provisioning) folder is loaded at
startup:

- **`globalrole-readonly.yaml`** — a `GlobalRole` named `readonly` with `read` on all
  scopes (`*`).
- **`globalrolebinding-mcp-readonly.yaml`** — a `GlobalRoleBinding` that binds the user
  `perses-mcp` (remember: username == Keycloak `client_id`) to the `readonly` role.
- **`project-*.yaml` / `dashboard-*.yaml`** — sample projects and dashboards so read calls
  return something meaningful.

Start Perses:

```bash
docker compose up -d perses
docker compose ps          # both containers should be "Up"
```

> Perses performs OIDC discovery against Keycloak **at startup**, so Keycloak must be
> ready first (Step 1). The compose file sets `restart: unless-stopped` on Perses as a
> safety net in case it starts before Keycloak is reachable.

---

## 5. Step 3 — Configure and run the Perses **Remote** MCP Server

The remote MCP server config is
[`permcp-http.yaml`](./oidc-keycloak-poc/permcp-http.yaml):

```yaml
transport: http            # <-- run as a REMOTE server (Streamable HTTP)
listen_address: ":8000"    # <-- endpoint: http://localhost:8000/mcp

perses_server:
  url: "http://localhost:8082"
  oauth:
    clientID: "perses-mcp"
    clientSecret: "perses-mcp-secret"
    # NOTE: tokenURL points at PERSES's own token endpoint, NOT Keycloak's.
    tokenURL: "http://localhost:8082/api/auth/providers/oidc/keycloak/token"
    authStyle: 2      # 2 = oauth2.AuthStyleInHeader -> send client_id/secret via HTTP Basic
```

Why these values:

- **`transport: http` + `listen_address: ":8000"`** make this a *remote* server. It serves
  the MCP Streamable HTTP endpoint at `http://localhost:8000/mcp`.
- **`tokenURL`** is the Perses `/token` endpoint (`.../oidc/{slug_id}/token`, where
  `slug_id` = `keycloak`). This is what makes Perses *broker* the credentials and return a
  Perses-native token, instead of the MCP server talking to Keycloak directly.
- **`authStyle: 2`** — Perses's `/token` handler reads the client credentials from the
  **HTTP Basic** `Authorization` header, so we force the OAuth2 client to send them that
  way.

Start the remote server (it reads its own config via `--config`; you start and own this
process — OpenCode does **not** launch it):

```bash
# from docs/oidc-keycloak-poc/
env -u PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN -u PERMCP_PERSES_SERVER_NATIVE_AUTH_PASSWORD \
  ../../bin/perses-mcp-server --config permcp-http.yaml
# (add `nohup ... > /tmp/permcp-http.log 2>&1 &` to run it in the background)
```

You should see `transport=http`, `Tools registered successfully`, and
`Perses MCP Server is running`.

> ⚠️ **Environment-variable precedence gotcha.** The MCP server reads env vars prefixed
> with `PERMCP_`, and **env vars override the YAML file**. If your shell has, for example,
> `PERMCP_PERSES_SERVER_NATIVE_AUTH_LOGIN` / `..._PASSWORD` exported (a common leftover
> from other setups), the server will see *two* auth methods configured and fail with
> `only one type of authentication should be configured`. The `env -u …` above strips them
> for the launched process; alternatively `unset` them in your shell.

---

## 6. Step 4 — Test the connection

### 6a. Verify the brokered auth + RBAC directly (curl → Perses)

This confirms the client-credentials brokering and read-only RBAC independently of the MCP
transport:

```bash
# Unauthenticated -> 401
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8082/api/v1/projects        # => 401

# Broker a Perses token via client-credentials (Basic auth to PERSES's /token endpoint)
TOKEN=$(curl -s -u perses-mcp:perses-mcp-secret \
  -d 'grant_type=client_credentials' \
  http://localhost:8082/api/auth/providers/oidc/keycloak/token | jq -r .access_token)

# Read works
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8082/api/v1/projects | jq '.[].metadata.name'

# Write is denied by RBAC (read-only technical user)
curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"kind":"Project","metadata":{"name":"should-fail"},"spec":{}}' \
  http://localhost:8082/api/v1/projects
# => {"message":"forbidden access: missing 'create' permission for 'Project' kind"}   (HTTP 403)
```

### 6b. Verify the remote MCP endpoint is up (curl → MCP server)

A quick `initialize` handshake against the Streamable HTTP endpoint confirms the remote
server is serving MCP:

```bash
curl -s -i -X POST http://localhost:8000/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}'
```

Expected: `HTTP/1.1 200 OK`, an `Mcp-Session-Id` response header, and an SSE `data:` line
containing `"serverInfo":{"name":"perses-mcp-server",...}`.

> A full `tools/call` over Streamable HTTP requires echoing the `Mcp-Session-Id` header on
> follow-up requests, so it's easiest to exercise tools through a real MCP client (next
> step) rather than by hand with curl.

### 6c. Connect OpenCode as a remote MCP client

Add the server to your OpenCode config (global `~/.config/opencode/opencode.json`, or a
project `opencode.json`). For a **remote** server OpenCode only stores the URL:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "perses-remote": {
        "type": "remote",
        "url": "http://localhost:8000/mcp",
        "oauth": false
      }
    }
  }
}
```

- **`type: "remote"`** + **`url`** — OpenCode connects over HTTP; it does not launch the
  process (you started it in Step 3).
- **`oauth: false`** — the MCP endpoint itself is not OAuth-protected. Auth to Perses is
  handled *inside* the server via its config; without this, OpenCode would try to run an
  OAuth flow against the MCP endpoint.

Confirm the connection:

```bash
opencode2 mcp list
# => perses-remote   connected
```

Then just ask OpenCode natural-language questions; it will call the `perses-remote_*`
tools. Examples:

- "List all projects in Perses"
- "What dashboards are in the observability project?"
- "Show me the frontend-health dashboard in web-platform"
- "Delete the poc-demo project"  → returns **403** (proves the read-only technical user)

---

## 7. Cleanup

```bash
# stop the remote MCP server (if backgrounded)
lsof -ti tcp:8000 | xargs -r kill

# stop containers and reset the Perses file DB
docker compose down
rm -rf perses/db/*

# optional: remove the OpenCode entry you added
# (restore your backup, or delete the "perses-remote" server from the config)
```

---

## 8. Production notes (what to change beyond the POC)

- **Do not hardcode secrets.** Use `perses_server.oauth.clientSecretFile` (or the
  `PERMCP_PERSES_SERVER_OAUTH_CLIENT_SECRET` env var) instead of an inline `clientSecret`,
  and mount the Keycloak client secret from a secret manager.
- **Use TLS** everywhere (Keycloak, Perses, and the MCP endpoint) and set proper
  `KC_HOSTNAME`/issuer values. Put the remote MCP endpoint behind HTTPS.
- **Give the technical user its own Keycloak client** (separate from Perses's relying-party
  client) so its identity/permissions are isolated. Perses supports a `client_credentials`
  override block on the OIDC provider for this.
- **Scope RBAC narrowly** — instead of `read` on `*`, grant only the resources/projects the
  MCP server actually needs.
- **Rotate** the client secret and rely on the automatic token refresh built into the flow.
