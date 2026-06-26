---
chunk_strategy: h1-h2-h3
description: ""
doc_type: research
ingested_at: "2026-06-26T02:33:59Z"
schema_version: "1.0"
source: docs/research/Backlogit-Architecture-Design.md
title: ""
---
# **Backlogit: Architecture & Design**

## **1\. Core Concept & Philosophy**

backlogit is a highly configurable, file-backed task management and agent operating system optimized for AI Agent consumption via the Model Context Protocol (MCP) and developer consumption via CLI/TUI. Built in **Golang**, it ships as a single, fast, dependency-free binary, making it trivial to drop into any CI/CD pipeline, agent environment, or developer machine instantly.

The fundamental tension in modern AI-assisted development is the conflict between human-centric tools and machine-centric requirements. Humans need readable, Git-friendly text files to collaborate asynchronously, review atomic diffs, and maintain absolute ownership over their project's state. AI agents, conversely, struggle with parsing thousands of verbose Markdown files. When forced to read raw files across large directories, agents rapidly exhaust context windows, incur massive API costs, and hallucinate relationships due to truncated context. backlogit resolves this tension by serving three primary purposes:

1. **Agile System of Record:** Stores tasks, OKRs, bugs, and decisions as individual Markdown files (with strictly typed YAML frontmatter). This ensures absolute compatibility with standard git workflows, branch-based pull request reviews, and simple text editors like Obsidian or VS Code. Furthermore, it syncs effortlessly with secondary systems of record (Jira, Azure DevOps) via external mapping, acting as a bidirectional bridge between local files and enterprise databases.  
2. **Agent Operating Context:** Acts as the persistent memory and telemetry sink for AI agents. Autonomous agents running in background loops often lose the "plot" of what they were trying to achieve. By logging their checkpoints, compacted semantic memories, and execution traces in machine-optimized, append-only formats, backlogit gives autonomous agents a long-term operational memory without bloating the primary human-facing codebase.  
3. **Legacy Evolution:** Provides seamless backward compatibility and automated migration from legacy backlog.md monolithic files and scattered folder structures. This allows agents to instantly take over existing, unstructured workspaces without requiring human developers to perform tedious manual data entry or disrupt their current in-flight sprints.

To maintain high performance and prevent agent context window bloat, backlogit employs a **Hybrid Data Architecture (CQRS)**, leveraging Markdown for current state, JSONL for historical event streaming, and an ephemeral SQLite database for instantaneous, token-efficient querying.

## **2\. Workspace Structure**

The tool operates within a .backlogit directory located at the root of the user's project workspace (CWD). The internal directory structure is **entirely user-defined** via registry.yaml. This is a critical design choice: it allows teams to segregate planning, execution, and agent telemetry in a way that matches their specific mental models, team structures, and operational governance.

Files are automatically routed and relocated to these directories by the core system based on their state mutations. For example, when an agent moves a ticket from todo to in\_progress, the file is physically moved on disk. This keeps the workspace visually clean, logically organized, and minimizes cognitive load for human developers browsing the file tree.

*Example of a comprehensive, user-defined structure:*

my-project/  
└── .backlogit/  
    ├── config.yaml          \# Defines artifact types, hierarchies, naming, and fields  
    ├── registry.yaml        \# Maps states/types to specific user-defined directories  
    ├── hooks.yaml           \# Configuration for external integrations  
    ├── index.db             \# Ephemeral SQLite cache/graph for instant SQL queries (Gitignored)  
    │  
    ├── planning/            \# OKRs, Milestones, and Feature Maps  
    │   ├── OKR01-q1-growth.md  
    │   └── M01-beta-launch.md  
    ├── epics/               \# High-level planning  
    │   └── E001-q1-auth-overhaul.md  
    ├── sprints/             \# Timeboxes and Sprint Goals  
    │   └── SPRINT-24.md  
    │  
    ├── sprint-board/        \# Active work (Stories, Tasks, Bugs)  
    │   ├── todo/  
    │   │   ├── US042-google-sso.md  
    │   │   └── BUG019-login-crash.md  
    │   └── active/  
    │       └── T105-implement-jwt.md  
    │  
    ├── knowledge/           \# Governance and Reviews  
    │   ├── decisions/       \# Architecture Decision Records (ADRs)  
    │   │   └── ADR005-use-jwt.md  
    │   └── reviews/         \# Code/Design Reviews  
    │  
    └── agent\_context/       \# High-volume AI machine data (Not Markdown)  
        ├── events.jsonl     \# Append-only ticket history & state changes  
        ├── telemetry.jsonl  \# Append-only execution logs and token usage  
        ├── checkpoints/     \# Session state snapshots  
        └── memories.json    \# Compacted vector/semantic memories

## **3\. Configuration Schemas**

### **config.yaml (Domain Model, Hierarchy & Custom Fields)**

The configuration file is the central nervous system of the backlogit workspace. The artifact\_types definition dictates the strict Agile hierarchy, explicitly outlining abbreviations, automated naming templates, and parent-child constraints. We've expanded this beyond simple tracking to include OKRs, bugs, and architecture decisions.

By enforcing the allowed\_children array, the system acts as a rigid, programmatic boundary for AI agents. An agent, due to a poorly phrased prompt, might accidentally attempt to assign a massive Epic as the child of a tiny Sub-task. If it attempts to execute this via the MCP tools, the core backlogit Go service rejects the transaction, returning a clear error message. This forces the agent to self-correct and strictly adhere to the team's defined Agile governance.

Furthermore, custom fields allow for seamless translation between human-readable local configurations and complex enterprise systems. Using the external\_map property, the abstraction is perfectly maintained. For example, a local user might simply specify priority: critical. When the background hook system syncs this to Azure DevOps, the external\_map automatically translates this into ADO's specific expected payload, such as {"op": "add", "path": "/fields/Microsoft.VSTS.Common.Severity", "value": "1 \- Critical"}.

\# Define the Agile hierarchy and naming conventions  
artifact\_types:   
  objective:  
    prefix: OKR  
    name\_format: "{prefix}{NNN}-{title\_slug}"  
    allowed\_children: \[key\_result, milestone, epic\]  
  milestone:  
    prefix: M  
    name\_format: "{prefix}{NNN}-{title\_slug}"  
    allowed\_children: \[epic, feature\]  
  epic:  
    prefix: E  
    name\_format: "{prefix}{NNN}-{title\_slug}"  
    allowed\_children: \[feature\]  
  feature:  
    prefix: F  
    name\_format: "{prefix}{NNN}-{title\_slug}"  
    allowed\_children: \[user\_story, bug\]  
  user\_story:  
    prefix: US  
    name\_format: "{prefix}{NNN}-{title\_slug}"  
    allowed\_children: \[task, sub\_task, issue\]  
  task:  
    prefix: T  
    name\_format: "{prefix}{NNN}"  
    allowed\_children: \[\]  
  bug:  
    prefix: BUG  
    name\_format: "{prefix}{NNN}-{title\_slug}"  
    allowed\_children: \[task\]  
  decision:  
    prefix: ADR  
    name\_format: "{prefix}{NNN}-{title\_slug}"  
    allowed\_children: \[\] \# ADRs stand alone but can be linked

fields:  
  type:  
    type: enum  
    values: $artifact\_types.keys()  
    default: task  
  status:  
    type: enum  
    values: \[todo, in\_progress, blocked, review, done, accepted, rejected\]  
    default: todo  
  parent:  
    type: string  
    optional: true  
  sprint:  
    type: string  
    description: "ID of the sprint this item belongs to (e.g., SPRINT-24)"  
    optional: true

## **4\. Planning & Timeboxes (Sprints/Milestones)**

Sprints and Milestones act as **containers** rather than just standard parent-child items. They provide crucial overarching context to the AI agent, serving as a focal point that helps minimize hallucinations and keep autonomous actions tightly aligned with current business priorities.

A sprint is defined by its own standalone Markdown file (e.g., sprints/SPRINT-24.md). The body of the markdown file explicitly holds the **Sprint Goal**, any relevant release notes, and specific target dates. Individual work items link to it via the sprint: SPRINT-24 frontmatter field.

When an agent queries the MCP for backlogit\_get\_sprint("SPRINT-24"), the tool queries the local cache to return the sprint goal *and* all artifacts currently assigned to that sprint. This contextual framing is incredibly powerful. It allows an agent to evaluate a newly discovered code issue and autonomously decide: "Does this bug threaten the stated goal of SPRINT-24? If yes, I will tag it as a blocker. If no, I will log it in the backlog for triage."

## **5\. Hybrid Data Architecture (CQRS)**

To balance Git compatibility with AI agent token efficiency, backlogit implements Command Query Responsibility Segregation (CQRS) across three distinct storage mechanisms. This architectural choice is the primary reason backlogit can scale to repositories with tens of thousands of tickets without crashing the AI's context window or incurring massive LLM API latency.

### **5.1 The Source of Truth: Markdown & YAML**

* **Purpose:** Git-friendly, human-readable primary storage used for version control and peer review.  
* **Rules:** Markdown files (.md) contain *only* the current state (via YAML frontmatter) and the current description. Historical comments, granular state changes, and lengthy agent "thought processes" are strictly forbidden from polluting these files. This isolation ensures that when an agent *does* need to read a file to understand a task, it only consumes a few hundred tokens rather than parsing a massive, unstructured history log.

### **5.2 The Query Engine: SQLite (index.db) & Auto-Rehydration**

* **Purpose:** Zero-context-bloat querying, instantaneous tree traversal, and complex relational filtering.  
* **Rules:** This is an *ephemeral cache* managed entirely by the backlogit core system. The .db file should be added to .gitignore. The AI agent **never** manually hydrates, creates tables, or manages this database schema.  
* **Auto-Rehydration Sync:** Because the index.db is strictly disposable, the system relies on a highly optimized Rehydration Engine. If index.db is accidentally deleted, if a developer performs a massive git pull that changes dozens of ticket files, or if files are edited manually in a separate IDE, the backlogit daemon automatically detects the drift. Leveraging Golang's lightweight goroutines, it quickly and concurrently walks the directory tree, parses all Markdown YAML frontmatter, and rebuilds the entire SQLite relational graph from scratch in milliseconds.  
* **Benefit:** Instead of an agent asking for "all tasks" and dumping 50,000 lines of JSON into its prompt, it executes a targeted tool call like backlogit\_query\_sql("SELECT id, title FROM items WHERE type='bug' AND status='in\_progress'"), receiving exactly the 50 tokens of data it requested. Furthermore, it utilizes SQLite's FTS5 (Full-Text Search) for lightning-fast keyword searches across the workspace, completely eliminating the need for agents to perform slow, iterative grep operations.

### **5.3 The Event Stream: JSONL (events.jsonl & telemetry.jsonl)**

* **Purpose:** Append-only history, state change auditing, comment threads, and agent operational logs.  
* **Rules:** When a status changes from todo to done, or when an agent adds an update comment to a ticket, a single JSON object is appended to events.jsonl containing the timestamp, actor, item\_id, and the delta.  
* **Benefit:** JSONL eliminates the risk of file corruption during concurrent agent writes. Agents can efficiently "tail" the JSONL file to only read the last 5 events for a specific ticket. This preserves massive amounts of context window while maintaining perfect, enterprise-grade audibility of every decision the AI (or human) has made.

## **6\. MCP Integration (AI Agent Interface)**

The Go binary exposes standard output/input (stdio) using the universal Model Context Protocol (MCP). By formalizing these tools, agents can logically chain actions together predictably. Because Go compiles to a native executable, startup time for the MCP server is practically zero, providing a much snappier experience for IDE-based agents.

**Tool Chaining Example:** An agent might use backlogit\_query\_sql to find an open ticket, use backlogit\_get\_item to read its exact requirements, execute a script to fix the code, use backlogit\_update\_item to move the status to "review", and finally use backlogit\_append\_comment to link the commit hash.

Exposed Tools for Agents:

* **Agile Management:**  
  * backlogit\_create\_item(type, title, description, parent\_id=None, sprint=None, \*\*kwargs)  
  * backlogit\_update\_item(item\_id, updates)  
  * backlogit\_get\_item(item\_id) (Returns the lean Markdown state).  
  * backlogit\_get\_item\_history(item\_id, limit=5) (Tails events.jsonl for recent activity).  
  * backlogit\_query\_sql(query) (Executes read-only, parameterized SQL against index.db for zero-bloat lookups).  
* **System Administration:**  
  * backlogit\_sync\_index() (Forces the system to rescan all Markdown files and rebuild the index.db cache. Typically triggered automatically, but available to agents to explicitly clear cache inconsistencies after manual git operations).  
* **Agent Operations:**  
  * backlogit\_append\_comment(item\_id, comment) (Writes to events.jsonl).  
  * backlogit\_log\_telemetry(event\_type, payload) (Writes to telemetry.jsonl).  
  * backlogit\_save\_memory(key, summary) (Updates memories.json).  
  * backlogit\_create\_checkpoint(state\_dump)

## **7\. Core Developer CLI & Initialization**

Before AI agents can operate autonomously within a workspace, the foundational structure must be scaffolded. backlogit provides a robust, developer-friendly set of CLI commands to manage the repository setup directly from the terminal.

### **7.1 Workspace Initialization (backlogit init)**

To onboard a repository into the backlogit ecosystem, the end user runs the backlogit init command at the root of their workspace. This command acts as the intelligent bootstrap sequence for the tool:

1. **Interactive Scaffolding:** It prompts the user for their preferred Agile methodology (e.g., "Scrum", "Kanban", or "Custom"). It may also ask about secondary systems: "Do you plan to sync with Jira or Azure DevOps?"  
2. **Configuration Generation:** Based on the user's selections, it creates the .backlogit/config.yaml and .backlogit/registry.yaml files, pre-populated with sensible, industry-standard defaults that can be tweaked later.  
3. **Directory Creation:** It creates the initial folder tree (like epics/, sprint-board/, and agent\_context/) precisely as defined in the generated registry rules.  
4. **Data Bootstrapping:** It initializes the empty index.db SQLite database and creates the blank events.jsonl streams, fully preparing the workspace for both human input and agent tool usage.

### **7.2 Core CLI Operations**

In addition to init, human developers have access to standard CLI operations that mirror the MCP tools, allowing for rapid terminal-based task management:

* backlogit create \--type user\_story \--title "Implement Login"  
  *Generates a new correctly-named markdown file, routes it to the proper folder, and updates the SQL index instantly.*  
* backlogit sync (Manually triggers the Auto-Rehydration Engine to aggressively rebuild the index.db from the markdown files. This is essential after pulling remote branches or resolving complex git merge conflicts).

### **7.3 AI Environment Registration (backlogit mcp init)**

To completely eliminate the friction of manually configuring MCP servers across various competing AI tools, backlogit includes an environment initialization command. Running backlogit mcp init \[environment\] automatically locates the host agent's hidden configuration file, safely parses the JSON, injects the backlogit stdio server configuration without overwriting existing settings, and saves it.

**Supported Environments & Configurations:**

* **VS Code (Extensions like Cline/Roo):** Safely injects into .vscode/mcp.json or the global IDE settings file.  
* **GitHub Copilot CLI:** Injects into .copilot/mcp-config.json.  
* **Cursor IDE:** Injects into .cursor/mcp.json.  
* **Claude Code:** Automatically executes the native CLI equivalent of claude mcp add backlogit \-- backlogit mcp.

*Example execution:*

$ backlogit mcp init ghcp  
\> Found GitHub Copilot config at .copilot/mcp-config.json  
\> Injected "backlogit" MCP server (Command: backlogit mcp)  
\> Success\! GitHub Copilot CLI can now manage your workspace.

## **8\. Future TUI Implementation**

By writing the core logic in **Golang**, the interface can be built natively for the terminal using **Bubble Tea** (https://github.com/charmbracelet/bubbletea), providing a highly concurrent, graphical experience without leaving the command line. Because of the CQRS architecture, the TUI completely bypasses slow disk I/O by querying the index.db SQLite database directly.

This allows the terminal UI to feel like a highly responsive native desktop app. It can render complex, deeply nested "Feature Maps" and dynamically filterable Sprint Boards instantaneously, even in workspaces with tens of thousands of archived tickets. Developers can envision a robust split-pane view with full Vim-keybinding support: a rich Agile Kanban board on the left, and a live-streaming "Agent Ops" console on the right tailing the telemetry.jsonl file. This gives engineering teams unprecedented, real-time insight into exactly what their autonomous coding agents are currently planning, thinking, and executing.

## **9\. Universal Agent & IDE Integrations (Slash Commands)**

To radically lower the barrier to entry, backlogit is explicitly designed to be invoked from within **any interactive AI development environment**—such as VS Code (Copilot Chat / Cline), Cursor, Claude Code, OpenAI Codex, or the GitHub Copilot CLI. It acts as a specialized setup agent and operational assistant, utilizing explicit slash commands to bridge the gap between natural language requests and the underlying structural configurations.

### **9.1 Workspace Scaffolding Skill (/backlogit-config)**

Instead of manually crafting complex config.yaml and registry.yaml files by hand, a developer can invoke the /backlogit-config skill to dynamically tailor the workspace. During this phase, the integration acts as an onboarding consultant; it performs a deep context analysis of the surrounding codebase to recommend the best agile mappings based on the detected frameworks.

**Example Invocations:**

* \> /backlogit-config Initialize a workspace for a Scrum team using Azure DevOps. We track Epics, Features, and Bugs, and we need a custom field for 'Story Points'.  
* \> /backlogit-config Set up a simple Kanban board layout with Jira sync. No Epics, just Stories and Tasks.

### **9.2 Operational Conversational Skills (/backlogit)**

Once initialized (either via backlogit init CLI or the /backlogit-config skill), developers can use the standard /backlogit slash command for daily work. Behind the scenes, the IDE integration acts as an intelligent intermediary. Before sending the user's prompt to the LLM, the extension executes optimal, predefined SQL queries against the local index.db cache. This injects perfect, token-efficient grounding data into the prompt invisibly. It prevents context overload and ensures the AI is fully aware of the current agile state before generating a response.

* \> /backlogit What's our current sprint goal and which P1 bugs are blocking it?  
* \> /backlogit Draft a new ADR to use Postgres instead of MySQL, link it to the current active Epic, and assign the ticket to me.  
* \> /backlogit I just finished the auth module. Find the related TODO tasks and move them all to the review column.

### **9.3 System & Legacy Migration Skills (/backlogit-sync, /backlogit-migrate)**

* \> /backlogit-sync (Manually triggers the Auto-Rehydration Engine from the chat interface, mirroring the CLI sync command).  
* \> /backlogit-migrate Target the legacy backlog.md in the root directory and automatically convert its contents to backlogit user stories and tasks.

## **10\. Legacy backlog.md Migration & Compatibility**

backlogit is intentionally designed to be dropped over existing, messy backlog.md workspace installations without causing immediate disruption or data loss. It includes a dedicated **Legacy AST (Abstract Syntax Tree) Parser** written in Go that understands common, implicit standard conventions from prior agent-based task tracking systems, translating human-formatted markdown into structured data.

### **10.1 In-Place Takeover (Read-Only Adapter)**

If a user simply runs backlogit init \--legacy, the tool will parse existing backlog.md monolithic files strictly in memory, **without modifying the underlying text files**. It generates the initial index.db SQLite database by heuristically interpreting the unstructured text:

* **Section Conventions:** Markdown headings like \# Backlog, \#\# In Progress, or \#\#\# Done are dynamically mapped to status fields within the database based on their structural position.  
* **Checklist Conventions:** Standard \[ \] Task Name and \[x\] Completed Task bullet points are mapped to todo and done states, automatically inferring completion.  
* **Legacy YAML:** Pre-existing frontmatter blocks in older setups are seamlessly mapped to the new schema constraints wherever possible.

This non-destructive approach builds trust. It allows an AI agent utilizing the new backlogit MCP tools to instantly query and interact with a legacy workspace on day one, without forcing an immediate migration.

### **10.2 The Transformation Pipeline**

When the team is comfortable and ready to fully adopt the new architecture, the user executes /backlogit-migrate (or the CLI equivalent backlogit migrate). The tool performs a safe, highly structured operational split:

1. **Extraction:** It reads the legacy monolithic backlog.md file (or legacy folders) into memory.  
2. **Decomposition:** It uses the Legacy AST Parser to slice the deeply nested markdown headings and bulleted task lists into distinct, atomic .md files.  
3. **Attribution:** It automatically infers hierarchy based on markdown depth (e.g., an \#\#\# H3 task nested underneath an \#\# H2 feature automatically becomes a sub\_task logically linked to a feature parent via the injected YAML frontmatter).  
4. **Scaffolding:** It writes these newly generated, atomic files into the proper .backlogit/ subdirectories according to the routing rules defined in registry.yaml. It then immediately triggers the Auto-Rehydration Engine to build the definitive index.db SQL cache.  
5. **Archiving:** It renames the original legacy file to backlog.legacy.md.bak to ensure absolute zero data loss, cleanly completing the evolution from a monolithic human file to a highly scalable, agent-native system of record.