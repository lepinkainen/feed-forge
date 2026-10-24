------------------------------ MODULE BulletinGenerate ------------------------------
(***************************************************************************)
(* Model of internal/bulletin: Fetch inserting items while one or more     *)
(* Generate executions read the unpublished backlog, call the model, and   *)
(* commit a bulletin with Store.CreateBulletin.                            *)
(*                                                                         *)
(* SharedLock: every Generate holds one lock for its whole run. TRUE is   *)
(*   the serve daemon (bulletinMu). FALSE is anything outside one process: *)
(*   cron overlap, or a manual `bulletin-generate` while `serve` runs.      *)
(* Guarded: CreateBulletin marks with                                      *)
(*   UPDATE ... WHERE id = ? AND bulletin_id IS NULL and rolls back when a *)
(*   row is already owned. FALSE is the current code (no guard).           *)
(***************************************************************************)
EXTENDS Naturals, FiniteSets

CONSTANTS Items, Gens, SharedLock, Guarded

None == 0

VARIABLES stored,     \* items present in the items table
          owner,      \* items.bulletin_id (None = unpublished)
          pc,         \* per-Generate program counter
          snap,       \* item IDs that UnpublishedItems returned to a Generate
          bulletins,  \* committed bulletins: [id, items summarised in content]
          lock,       \* holder of the shared lock, or "free"
          llmCalls,   \* model calls made (spend)
          nextId

vars == <<stored, owner, pc, snap, bulletins, lock, llmCalls, nextId>>

Init ==
  /\ stored = {}
  /\ owner = [i \in Items |-> None]
  /\ pc = [g \in Gens |-> "idle"]
  /\ snap = [g \in Gens |-> {}]
  /\ bulletins = {}
  /\ lock = "free"
  /\ llmCalls = 0
  /\ nextId = 1

\* bulletin.Fetch -> Store.InsertItem (ON CONFLICT DO NOTHING).
Fetch(i) ==
  /\ i \notin stored
  /\ stored' = stored \cup {i}
  /\ UNCHANGED <<owner, pc, snap, bulletins, lock, llmCalls, nextId>>

Start(g) ==
  /\ pc[g] = "idle"
  /\ IF SharedLock THEN lock = "free" /\ lock' = g ELSE UNCHANGED lock
  /\ pc' = [pc EXCEPT ![g] = "read"]
  /\ UNCHANGED <<stored, owner, snap, bulletins, llmCalls, nextId>>

Release == IF SharedLock THEN lock' = "free" ELSE UNCHANGED lock

\* Store.UnpublishedItems: SELECT ... WHERE bulletin_id IS NULL.
Read(g) ==
  /\ pc[g] = "read"
  /\ LET u == {i \in stored : owner[i] = None} IN
       IF u = {}
         THEN /\ pc' = [pc EXCEPT ![g] = "done"] /\ Release
              /\ UNCHANGED snap
         ELSE /\ snap' = [snap EXCEPT ![g] = u]
              /\ pc' = [pc EXCEPT ![g] = "summarise"]
              /\ UNCHANGED lock
  /\ UNCHANGED <<stored, owner, bulletins, llmCalls, nextId>>

\* Summarizer.Summarize: one model call (tens of seconds in reality).
Summarise(g) ==
  /\ pc[g] = "summarise"
  /\ llmCalls' = llmCalls + 1
  /\ pc' = [pc EXCEPT ![g] = "commit"]
  /\ UNCHANGED <<stored, owner, snap, bulletins, lock, nextId>>

\* Store.CreateBulletin: one SQLite transaction, so atomic here.
Commit(g) ==
  /\ pc[g] = "commit"
  /\ IF Guarded /\ \E i \in snap[g] : owner[i] # None
       THEN UNCHANGED <<owner, bulletins, nextId>>   \* rollback
       ELSE /\ owner' = [i \in Items |-> IF i \in snap[g] THEN nextId ELSE owner[i]]
            /\ bulletins' = bulletins \cup {[id |-> nextId, items |-> snap[g]]}
            /\ nextId' = nextId + 1
  /\ pc' = [pc EXCEPT ![g] = "done"]
  /\ Release
  /\ UNCHANGED <<stored, snap, llmCalls>>

Next ==
  \/ \E i \in Items : Fetch(i)
  \/ \E g \in Gens : Start(g) \/ Read(g) \/ Summarise(g) \/ Commit(g)

Spec == Init /\ [][Next]_vars

TypeOK == lock \in Gens \cup {"free"} /\ llmCalls \in 0..Cardinality(Gens)

\* The digest stored in a bulletin describes exactly the items that belong to it.
ContentMatchesOwnership ==
  \A b \in bulletins : \A i \in b.items : owner[i] = b.id

\* No story is published in two bulletins.
NoDoubleSummary ==
  \A b1, b2 \in bulletins : b1.id # b2.id => b1.items \cap b2.items = {}

\* "Never run two bulletin-generate executions concurrently: both would
\* summarize the same backlog and double the model spend." (CLAUDE.md)
NoWastedModelCall == llmCalls <= Cardinality(bulletins) + Cardinality({g \in Gens : pc[g] = "commit"})
=============================================================================
