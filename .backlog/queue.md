# Queue

- Create all the backlogit commands needed to fully manage a backlog
	- add (with flag for type)
	- init
	- ... and many other commands (research and suggest commands to add)

- Enable YAML header definition via header-def.yaml with immutable default items by type
	- type (enums: Epic, Feature, Sub-Epic, User-Story, Task, Sub-Task, Bug, Decision, )
	- created_date
	- updated_date
	- id (prefix OP, short for Opperation or Opus, and 3 digit numeral: OP001)
	- title
	- status (enums: To-Do, In-Progress, Blocked, Done)
	- assigned-to
	- owner
	- labels
	- dependencies []
	- references []
	- priority (enums: Low, Medium, High)
	- parent-id
	- commit

- Create templates of many common operation types, such as tasks, bugs, features, etc.
- Each template will live in a .backlog/templates directory.
- A registry.yaml file will define which templates are in use by the current workspace
- IDs are immutable by hand; they must only be modified using the backlogit tool to reparent, for example.
- To optimize markdown write of content, backlogit must be built such that it can accept multi-line markdown text via an input buffer for each section.
- Sections can be updated with tool use by section flag, which will be defined in the template for each section.
- Customized tool calls are then discoverable by the agent as backlogit generates the tool call list with descriptions based on the registered templates.
- Custom task sections by type
	- Each template will include the body section BEGIN/END section tags that will allow backlogit to programmatically identify the section within the markdown file.
