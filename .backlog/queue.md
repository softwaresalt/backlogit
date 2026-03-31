# Queue

- Customizations to templates and YAML definitions shoud automatically cause modifications to the DB schema; schema should be directly mapped to YAML definitions.
- Must be able to define for a Work Item Type (WIT) template whether a field or attribute or template section is required or optional.
- Backlogit must be able to return metadata about specific work item types (WIT) that include whether the field is required or optional.
- Templates must include description entries within each template of the template itself, WIT relationship description to other WITs, and description elements for each attribute.
- Big idea is that agent should be able to query backlogit for list of WITs, their relationships to each other, required and optional fields, sections, attributes, and enums in order to allow the agent to effectively decipher and navigate the workflow.
- Must have command for archiving completed work (tasks, features, epics, bugs) once the associated branch has been merged to the main branch.
- Must be able to track commits related to work items such that backlogit can programmatically deteremine that work, plans, and research, for example, have been fully completed and auto-archive the items into the .backlogit/archive/subdirectory associated with the template.
- WIT templates must also include directory map of where instances of this populated template will be manifested, tracked and also where archived.
- Backlog it must be able to list queued, active, blocked, done, and other enum status levels of work items of different types in a tabular format in the console window when queried from the command line.  Further, when asked to "view" and item by ID, backlogit should render the full text and YAML header of the work item/product in the console.
