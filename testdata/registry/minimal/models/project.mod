name: Project
fields:
  ID:
    type: UUID
  Code:
    type: String
  Name:
    type: String
  Description:
    type: String
    attributes:
      - optional
identifiers:
  primary: ID
  code: Code
related:
  Organization:
    type: ForOne
