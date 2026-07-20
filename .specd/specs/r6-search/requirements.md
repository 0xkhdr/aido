# Requirements — r6-search

> Global search for tasks by title and description across all projects.

## R6.5 — Search across projects and tasks

owner: craftsman
priority: should
risk: low

- **R6.5.1** When user enters a search query in the search box (in navigation bar) and submits, the system shall search all tasks (across all projects) by title and description and display matching results.
- **R6.5.2** When search results are displayed, the system shall show each matching task with its project name and a link to the task detail page.
- **R6.5.3** When search query returns no matches, the system shall display "No tasks found" message.
- **R6.5.4** When search query is empty or contains special characters, the system shall perform literal substring match (no regex or operators).

## Edge and failure behavior

- Empty query (q=""): display empty results or redirect to home
- Query too long (>1000 chars): reject with 400 or truncate silently
- Search is case-insensitive
- Results limited to reasonable count (e.g., 100 results) for performance
- Special characters (quotes, asterisks, etc.): treated as literal text, not operators

## Non-goals

- Search filters by project, priority, due date
- Full-text search with ranking/relevance scoring
- Search history or saved searches
- Real-time search suggestions/autocomplete
- Search by tag (separate from task title/description)
- Regex or advanced query syntax
