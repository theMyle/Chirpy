## Connection String
- "postgres://postgres:postgres@localhost:5432/chirpy"

## SQL

If something must be `NOT NULL`, and the `system` is responsible for it, provide a system generated `DEFAULT`. Else if it's `user supplied` then you can leave it without `DEFAULT` as validation check.

Table Column Format - `[NAME]` `[TYPE]` `[MODIFIERS]`

Example:

```sql
id UUID PRIMARY KEY DEFAULT gen_random_uuid()
```

### Using Goose to for migration

```bash
goose -dir [dir] [driver] [connection string] up    # up migration
goose -dir [dir] [driver] [connection string] down  # down migration
```


