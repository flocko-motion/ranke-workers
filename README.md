# Ranke Workers

Workers for [RankeDB](https://github.com/flocko-motion/rankedb) — external processes that read and write the knowledge graph through the API.

Each worker lives in its own directory with its own `go.mod`.

## Workers

| Worker | Purpose |
|--------|---------|
| [google](google/) | Google Takeout formats (Contacts, Gmail, Calendar, ...) |

## Building Workers

See the [RankeDB Workers Manual](https://raw.githubusercontent.com/flocko-motion/rankedb/refs/heads/main/WORKERS.md) for how to build workers against the RankeDB API.
