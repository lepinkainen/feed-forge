# Feed Forge — Project Purpose

This document tells you why the project exists and how to keep work in its
scope. `CLAUDE.md` gives the build commands, the architecture, and the
conventions. This document does not repeat them.

## What it is

Feed Forge makes Atom feeds for sites that have no feed, a bad feed, or too
much traffic to read item by item. It turns a large flow of items into a feed
that a feed reader can show calmly. The output is standard Atom with no custom
namespaces. As a result, all feed readers can parse it.

The second part of the project is the **bulletin pipeline**. This pipeline
collects many news feeds and removes near-duplicate stories. Then it publishes
one summarized digest two times each day, not hundreds of headlines.

## Scope — a single-user personal project

One person runs this project on local machines. Make your design decisions for
that condition. A feature that helps only many users or an operations team is
out of scope. Do not add these features:

- User accounts, authentication, multi-tenancy, or per-user settings.
- A database migration framework. The databases are small, and the user can
  build them again. When a schema changes, give the user a one-off migrate
  command.
- Horizontal scaling, job queues, or service infrastructure.

If you are not sure, select the simplest design that works for one person.

## Direction

A new source must stay cheap to add: one package in `internal/`, the provider
interface, and one registration. Put your effort into the dedup quality and
the summarization quality of the bulletin pipeline.
