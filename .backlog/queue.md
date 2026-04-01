# Queue

- First order of business: there are two backlog items with id TASK-003, two with TASK-004, TASK-005, TASK-006, and TASK-007.  Some of these are bugs, others tasks. I want to reorganize these tasks and bugs into a single set. I want you to reorganize these such that TASK-003 will be a feature level task.  The existing tasks should be arranged as tasks and bugs that have TASK-003 as their parent.

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

- Rather than having a subfolder of .backlogit named tasks, we should generalize this folder since it will hold a range of work item types, so the folder should be named instead .backlogit\queue.  The way we will organize work items within the folder will be such that the file names will be named hierarchically.  The default hierarchy of work item types will be: level 1 => 001, level 2 => 001.001, level 3 => 001.001.001.  The default will be level 1 = feature type, level 2 = task type, level 3 = subtask type.  But the user can reorganize the work item types to link to the hierarchy level they want.  They could choose to set level 1 = epic type, for example.  They could establish 4 levels. They could establish both tasks and bugs as level 2 work items.  This way, the ID and the file naming conventions can remain static and programmable.
