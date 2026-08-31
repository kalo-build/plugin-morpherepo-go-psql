name: Task
fields:
  ID:
    type: UUID
  Title:
    type: String
  Status:
    type: TaskStatus
identifiers:
  primary: ID
related:
  Project:
    type: ForOne
    attributes:
      - optional
