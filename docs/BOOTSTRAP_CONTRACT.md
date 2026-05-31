# Generic Internal Bootstrap Contract

This repository provides public CLI primitives only. Organization-specific
installation and enrollment should live in an internal wrapper outside this
public repo.

Use neutral wrapper names in public docs and examples, such as:

```text
install-company-outlook-agent
```

Do not publish real company names, domains, internal URLs, registry names,
private config schemas, or tenant bootstrap commands here.

## Internal Wrapper Responsibilities

An internal wrapper may:

1. Install a signed or otherwise trusted `outlook-agent` binary.
2. Create private config outside the public repository.
3. Provision the approved secret store.
4. Run `outlook-agent doctor`.
5. Run `outlook-agent auth check --config <private-config>`.
6. Run `outlook-agent setup agent apply --client <client> --scope <scope> --config <private-config> --yes --backup`.
7. Run an MCP smoke check in the target agent host.

The wrapper must not write secrets to a repository, shell profile, shell
history, plugin package, generated skill file, or public documentation.

Approval secrets should stay in a trusted host/operator environment and should
not be exposed to the agent conversation context.
