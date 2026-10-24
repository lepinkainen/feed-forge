-------------------------------- MODULE SlotLoop --------------------------------
(***************************************************************************)
(* Model of daemon.slotLoop / nextSlotFire in cmd/feed-forge/serve.go.     *)
(* Time is discrete: one unit is 15 s, so Minute = 4 units.               *)
(*  w : wall clock (time.Now, what nextSlotFire and target.Sub see)        *)
(*  m : monotonic clock (what time.After measures)                         *)
(* Slots are positions in a Day of DayLen units.                           *)
(***************************************************************************)
EXTENDS Integers, FiniteSets

CONSTANTS DayLen,      \* units per day
          Minute,      \* slotRecheckInterval in units
          Horizon,     \* wall-clock bound for model checking
          SlotChoices, \* possible slot sets (initial config and reloads)
          MaxDigest,   \* longest digest run in units
          MaxReloads,  \* SIGHUP reloads allowed
          MaxStepBack  \* total units the wall clock may be stepped back (NTP)

VARIABLES w, m, pc, wakeAt, fire, target, digestEnd,
          slots, reloads, lastReload, stepped,
          fireCount,   \* target occurrence -> number of digests fired for it
          missed, coalesced

vars == <<w, m, pc, wakeAt, fire, target, digestEnd, slots, reloads,
          lastReload, stepped, fireCount, missed, coalesced>>

Occ == 0..(Horizon + DayLen)

\* nextSlotFire: the first wall time strictly after now whose slot is configured.
NextSlotFire(now, S) ==
  CHOOSE t \in (now + 1)..(now + DayLen) :
    /\ (t % DayLen) \in S
    /\ \A u \in (now + 1)..(t - 1) : (u % DayLen) \notin S

Init ==
  /\ w = 0 /\ m = 0
  /\ pc = "compute"
  /\ wakeAt = 0 /\ fire = FALSE /\ target = 0 /\ digestEnd = 0
  /\ slots \in SlotChoices
  /\ reloads = 0 /\ lastReload = -Minute
  /\ stepped = 0
  /\ fireCount = [t \in Occ |-> 0]
  /\ missed = FALSE /\ coalesced = FALSE

\* Top of the loop: cfg := d.cfg.Load(); now := d.now(); decide wait/fire.
Compute ==
  /\ pc = "compute"
  /\ IF slots # {} /\ NextSlotFire(w, slots) - w <= Minute
       THEN /\ target' = NextSlotFire(w, slots)
            /\ wakeAt' = m + (target' - w)
            /\ fire' = TRUE
       ELSE /\ wakeAt' = m + Minute
            /\ fire' = FALSE
            /\ UNCHANGED target
  /\ pc' = "sleep"
  /\ UNCHANGED <<w, m, digestEnd, slots, reloads, lastReload, stepped,
                 fireCount, missed, coalesced>>

\* <-time.After(wait) returned. If fire: runDigest(ctx, d.cfg.Load()).
Wake ==
  /\ pc = "sleep" /\ m >= wakeAt
  /\ IF fire
       THEN /\ pc' = "digest"
            /\ fireCount' = [fireCount EXCEPT ![target] = @ + 1]
            /\ \E d \in 0..MaxDigest : digestEnd' = m + d
       ELSE /\ pc' = "compute"
            /\ UNCHANGED <<fireCount, digestEnd>>
  /\ UNCHANGED <<w, m, wakeAt, fire, target, slots, reloads, lastReload,
                 stepped, missed, coalesced>>

DigestDone ==
  /\ pc = "digest" /\ m >= digestEnd
  /\ pc' = "compute"
  /\ UNCHANGED <<w, m, wakeAt, fire, target, digestEnd, slots, reloads,
                 lastReload, stepped, fireCount, missed, coalesced>>

\* A slot occurrence at wall time w is due. Time may only pass while the loop
\* is blocked, so an on-time Wake for w has already happened by now.
Owed(t) == (t % DayLen) \in slots /\ fireCount[t] = 0

Tick ==
  /\ w < Horizon
  /\ \/ pc = "sleep" /\ m < wakeAt
     \/ pc = "digest" /\ m < digestEnd
  /\ w' = w + 1 /\ m' = m + 1
  /\ missed' = (missed \/ (w > 0 /\ Owed(w) /\ pc = "sleep" /\ lastReload + Minute < w))
  /\ coalesced' = (coalesced \/ (w > 0 /\ Owed(w) /\ pc = "digest"))
  /\ UNCHANGED <<pc, wakeAt, fire, target, digestEnd, slots, reloads,
                 lastReload, stepped, fireCount>>

\* SIGHUP with a valid file: d.cfg.Store(new snapshot).
Reload ==
  /\ reloads < MaxReloads
  /\ \E S \in SlotChoices : slots' = S
  /\ reloads' = reloads + 1 /\ lastReload' = w
  /\ UNCHANGED <<w, m, pc, wakeAt, fire, target, digestEnd, stepped,
                 fireCount, missed, coalesced>>

\* NTP (or an operator) steps the wall clock back; monotonic time is unaffected.
StepBack ==
  /\ stepped < MaxStepBack /\ w > 0
  /\ w' = w - 1 /\ stepped' = stepped + 1
  /\ UNCHANGED <<m, pc, wakeAt, fire, target, digestEnd, slots, reloads,
                 lastReload, fireCount, missed, coalesced>>

Next == Compute \/ Wake \/ DigestDone \/ Tick \/ Reload \/ StepBack

Spec == Init /\ [][Next]_vars

\* A slot that has been configured for at least one recheck interval fires,
\* unless the previous digest is still running.
NoMissedSlot == ~missed

\* Each slot occurrence produces at most one digest.
NoDoubleFire == \A t \in Occ : fireCount[t] <= 1

\* A slot occurrence never passes while the previous digest still runs.
NoCoalescedSlot == ~coalesced
=============================================================================
