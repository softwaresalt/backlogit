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

- A huge requirement of the backlog is its ability to track dependencies between work items at the:
  
  - feature level
  - epic level
  - task level
  - sub-task level
  - bugs
  - decisions

- Another huge requirement of the backlog is that it must be able to produce a queue of work, a streaming queue of work, for the agent so that all the agent has to do is go to the backlog and effectively ask it, "What is the next thing to work on?" At the feature level, at the epic level, at the task level, it must be able to do this for the development stage so that would be for the build orchestrator, for example, to go through and for the PR review. The key thing is that again it must be able to simply query the database using the backlog programmatically to identify what to work on next and what are the dependencies so that it knows what it can run in parallel and what it can't. 

- Need to include new status attribute for feature level WIT.  The core requirement is to enable the workflow policy primitive of the agent harness to enforce state change control over when and how work items progress through the harness workflow.  There may be additional attributes needed to enforce these policies, which will be found in the .github/policies directory.
  
  - Attribute: harness
  
  - Status enums: ready, (other status levels)


