# skpm Registry

> The package registry and API powering skpm.

This repository serves two purposes: it's the HTTP API that the CLI and plugin talk to, and it's the data store — package metadata and script files live here as version-controlled JSON.

---

## API

Base URL: `https://skpm-registry-production.up.railway.app`

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/packages` | List all packages |
| `GET` | `/packages/:name` | Get metadata for a package (all versions) |
| `GET` | `/packages/:name/versions/:version/files/:file` | Download a script file |
| `GET` | `/search?q=` | Search packages by name or description |
| `POST` | `/publish` | Publish a new package or version |

### Package format

Each package is stored as `packages/<name>.json`:

```json
{
  "name": "economy",
  "description": "A Vault-backed economy system",
  "author": "adammcgrogan",
  "latest": "1.0.1",
  "versions": {
    "1.0.1": {
      "skript": ">=2.8.0",
      "minecraft": ">=1.20",
      "addons": {},
      "files": [
        {
          "name": "economy.sk",
          "url": "https://raw.githubusercontent.com/skpm-dev/registry/main/files/economy/1.0.1/economy.sk",
          "sha256": "sha256:7k2j..."
        }
      ]
    }
  }
}
```

---

## Publishing

Packages are published via the [skpm CLI](https://github.com/skpm-dev/cli), not directly to this repo. The CLI sends your manifest and files to `POST /publish`, which opens a pull request here. A maintainer reviews and merges it — at that point the package is live.

Publishing requires a GitHub personal access token passed as `Authorization: Bearer <token>`. Ownership is enforced: only the original author can publish new versions of a package.

---

## Repository structure

```
registry/
├── index.json              ← searchable index of all packages
├── packages/
│   ├── economy.json
│   └── join-message.json
└── files/
    ├── economy/
    │   └── 1.0.1/
    │       └── economy.sk
    └── join-message/
        └── 1.0.0/
            └── join-message.sk
```

---

## Related

- [skpm-dev/cli](https://github.com/skpm-dev/cli) — CLI tool for publishing packages
- [skpm-dev/plugin](https://github.com/skpm-dev/plugin) — Bukkit plugin for installing packages in-game
