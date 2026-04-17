# Ranke Workers — Claude Code Instructions

## Project overview

This repo contains workers for [RankeDB](https://github.com/flocko-motion/rankedb). Workers are external processes that read and write the knowledge graph through the API using the worker SDK (`github.com/flocko-motion/rankedb/worker`).

## Golden rule

If a feature is missing in the RankeDB SDK: **ask for it** — file an issue at `flocko-motion/rankedb`. Don't work around it. Don't skip it. Wait for the fix.

## Documentation

- Worker SDK manual: https://raw.githubusercontent.com/flocko-motion/rankedb/refs/heads/main/WORKERS.md
- RankeDB paper (architecture spec): https://raw.githubusercontent.com/flocko-motion/rankedb/refs/heads/main/papers/01-rankedb.md

## Structure

- `types/` — shared normalized interchange formats (`text/ranke` encoding), organized by level (l0, l1, l2)
- `google/` — Google Takeout worker (contacts, and more formats coming)

## text/ranke encoding

All normalized L0 types use a shared plaintext format defined in `types/`:
- Headers: `Key: Value` or `Key (label): Value`, one per line
- Body: freeform text after a blank line
- Domain types (e.g. `l0.Contact`) marshal/unmarshal to this format via `ToText()`/`FromText()`
