The `memory` tool allows you to store and retrieve information across conversations for long-term knowledge retention.

## Actions

### remember
Store a new memory or update an existing one.
- **Required**: `key`, `value`
- **Optional**: `scope` (session/project/global), `metadata`

Example:
```json
{
  "action": "remember",
  "key": "project_architecture",
  "value": "This project uses a microservices architecture with Go backend and React frontend",
  "scope": "project",
  "metadata": {"category": "architecture", "priority": "high"}
}
```

### recall
Retrieve a previously stored memory.
- **Required**: `key`
- **Optional**: `scope` (session/project/global)

Example:
```json
{
  "action": "recall",
  "key": "project_architecture",
  "scope": "project"
}
```

### forget
Remove a memory.
- **Required**: `key`
- **Optional**: `scope` (session/project/global)

Example:
```json
{
  "action": "forget",
  "key": "old_decision",
  "scope": "project"
}
```

### list
List all memories in a scope.
- **Optional**: `scope` (session/project/global)

Example:
```json
{
  "action": "list",
  "scope": "project"
}
```

## Scopes

- **session**: Memory lasts only for the current conversation session
- **project**: Memory persists across sessions for this project (working directory)
- **global**: Memory persists across all projects (defaults to project if not specified)

## Use Cases

- Remember architectural decisions
- Store commonly used commands or patterns
- Track project-specific conventions
- Maintain a knowledge base of learnings
- Remember user preferences across sessions

## Best Practices

- Use descriptive keys (e.g., "deployment_process" not "dp")
- Add metadata tags for easier organization
- Use project scope for project-specific knowledge
- Use global scope for general learnings or cross-project patterns
- Regularly review and clean up outdated memories with `list` and `forget`
