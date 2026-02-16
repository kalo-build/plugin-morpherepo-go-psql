name: Task
fields:
  ID:
    type: UUID
  Title:
    type: String
  Status:
    type: String
identifiers:
  primary: ID
related:
  Project:
    type: ForOne
