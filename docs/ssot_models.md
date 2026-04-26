# SSOT Model Guard

This tool (`tools/modelguard`) ensures that the Client and Server domain models remain synchronized.
Since Go does not support sharing code natively between a full Go server and a GopherJS/WASM client easily in this specific monorepo structure without modules, we maintain two copies of `models.go`.

This guardrail prevents "drift" (when a field is added to the Server but forgotten on the Client, or vice-versa).

## Usage

From the repository root:

```bash
go run ./tools/modelguard
```

## How it Works
1. Parses `server/internal/domain/models.go` and `client/internal/domain/models.go`.
2. Inspects strict list of shared struct: `Player`, `Island`, `Fleet`, `Ship`, etc.
3. Compares the **JSON Keys** (`json:"key"`).
4. Ignores fields with `json:"-"`.

## Outputs
- **EXIT 0 (Success)**: No missing fields. (Warnings about type mismatches may appear).
- **EXIT 1 (Failure)**: Missing fields detected (Critical Drift).

## Fixing Drifts
If the tool fails:
1. Read the output to see which fields are missing.
2. Update the outdated file (usually Client) to match the Source of Truth (Server).
3. Ensure types match (e.g., `uuid.UUID` on server corresponds to `string` or `uuid.UUID` on client).
