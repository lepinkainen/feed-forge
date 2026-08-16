# Feed Forge — Project Purpose

This document says *why* the project exists and how to stay aligned with its
scope. Build commands, architecture, and conventions live in `CLAUDE.md`; this
file does not repeat them.

## What it is

A generator of Atom feeds for sites that have no feed, a bad feed, or far too
much traffic to read item by item. It turns a firehose into something a feed
reader can present calmly. Output stays standard Atom with no custom namespaces,
so any reader can parse it.

The second half of the idea is the **bulletin pipeline**: collect many news
feeds, drop near-duplicate stories, and publish one summarized digest twice a
day instead of hundreds of headlines.

## Scope — a single-user personal project

This runs for one person on local machines. Optimise for that reality and treat
anything that only pays off for many users or an operations team as out of
scope. In particular, do not add:

- User accounts, authentication, multi-tenancy, or per-user settings.
- An elaborate database migration framework. The databases are small,
  single-user, and rebuildable; hand over a one-off migrate command when a
  schema changes instead of building migration machinery.
- Horizontal scaling, job queues, or service infrastructure.

When in doubt, prefer the simplest thing that works for one person.

## Direction

Adding a source should stay cheap: a package under `internal/`, the provider
interface, one registration. Effort belongs in the bulletin pipeline's dedup and
summarization quality.
