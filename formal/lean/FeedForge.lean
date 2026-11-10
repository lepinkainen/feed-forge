/-
  Lean 4 models of pure algorithms in feed-forge. Core Lean only; check with
  `lean FeedForge.lean`.

  1. nextSlotFire (cmd/feed-forge/appconfig.go): time is minutes since a local
     midnight epoch, no DST (Go's AddDate keeps the wall time across DST; that
     part is outside this model).
  2. clusterItems (internal/bulletin/dedup.go): the clusters partition the input.
  3. SimpleRateLimiter.WaitContext (pkg/api/ratelimit.go): grants keep the
     spacing, but a cancelled waiter keeps its reservation.
-/

/-! ## 1. nextSlotFire -/

/-- One slot's candidate: today's occurrence, or tomorrow's if not after now. -/
def cand (now s : Nat) : Nat :=
  if now / 1440 * 1440 + s ≤ now then now / 1440 * 1440 + s + 1440
  else now / 1440 * 1440 + s

/-- The Go loop keeps the earliest candidate. -/
def nextSlotFire (now : Nat) : List Nat → Option Nat
  | [] => none
  | s :: ss =>
    match nextSlotFire now ss with
    | none => some (cand now s)
    | some n => some (min (cand now s) n)

theorem day_bounds (now : Nat) : now / 1440 * 1440 ≤ now ∧ now < now / 1440 * 1440 + 1440 :=
  ⟨Nat.div_mul_le_self now 1440, Nat.lt_div_mul_add (by decide)⟩

theorem cand_gt (now s : Nat) : now < cand now s := by
  unfold cand; obtain ⟨h1, h2⟩ := day_bounds now; generalize now / 1440 * 1440 = q at *
  by_cases hc : q + s ≤ now <;> simp only [hc, ↓reduceIte] <;> omega

theorem cand_le (now s : Nat) (h : s < 1440) : cand now s ≤ now + 1440 := by
  unfold cand; obtain ⟨h1, h2⟩ := day_bounds now; generalize now / 1440 * 1440 = q at *
  by_cases hc : q + s ≤ now <;> simp only [hc, ↓reduceIte] <;> omega

theorem cand_mod (now s : Nat) (h : s < 1440) : cand now s % 1440 = s := by
  unfold cand
  have hq : now / 1440 * 1440 % 1440 = 0 := Nat.mul_mod_left _ _
  generalize now / 1440 * 1440 = q at *
  by_cases hc : q + s ≤ now <;> simp only [hc, ↓reduceIte] <;> omega

theorem cand_first (now s t : Nat) (h : s < 1440) (h1 : now < t) (h2 : t < cand now s) :
    t % 1440 ≠ s := by
  unfold cand at h2
  obtain ⟨b1, b2⟩ := day_bounds now
  have hq : now / 1440 * 1440 % 1440 = 0 := Nat.mul_mod_left _ _
  generalize now / 1440 * 1440 = q at *
  by_cases hc : q + s ≤ now <;> simp only [hc, ↓reduceIte] at h2 <;> omega

/-- The result is strictly after now, at most one day away, lands on a
    configured slot, and no configured slot occurs before it. -/
theorem nextSlotFire_correct (now : Nat) :
    ∀ (slots : List Nat), (∀ s ∈ slots, s < 1440) → ∀ r, nextSlotFire now slots = some r →
      now < r ∧ r ≤ now + 1440 ∧ r % 1440 ∈ slots ∧
      ∀ t, now < t → t < r → t % 1440 ∉ slots := by
  intro slots
  induction slots with
  | nil => intro _ r h; simp [nextSlotFire] at h
  | cons s ss ih =>
    intro hs r h
    have hs0 : s < 1440 := hs s (by simp)
    have hss : ∀ x ∈ ss, x < 1440 := fun x hx => hs x (by simp [hx])
    simp only [nextSlotFire] at h
    split at h
    · next hn =>
      cases h
      refine ⟨cand_gt now s, cand_le now s hs0, by simp [cand_mod now s hs0], ?_⟩
      intro t h1 h2 hmem
      simp only [List.mem_cons] at hmem
      rcases hmem with heq | hin
      · exact cand_first now s t hs0 h1 h2 heq
      · -- ss is nonempty here, so its result would exist
        cases ss with
        | nil => simp at hin
        | cons x xs => simp [nextSlotFire] at hn; split at hn <;> simp at hn
    · next n hn =>
      cases h
      obtain ⟨g, le, mem, first⟩ := ih hss n hn
      refine ⟨by have := cand_gt now s; omega, by omega, ?_, ?_⟩
      · by_cases hc : cand now s ≤ n
        · simp [Nat.min_eq_left hc, cand_mod now s hs0]
        · simp [Nat.min_eq_right (by omega : n ≤ cand now s), mem]
      · intro t h1 h2 hmem
        simp only [List.mem_cons] at hmem
        rcases hmem with heq | hin
        · exact cand_first now s t hs0 h1 (by omega) heq
        · exact first t h1 (by omega) hin

/-! ## 2. clusterItems -/

section Cluster
variable {α : Type} (near : α → α → Bool) (zero : α → Bool)

/-- Append `it` to the first cluster whose representative (head) is near it. -/
def place (it : α) : List (List α) → Option (List (List α))
  | [] => none
  | [] :: cs => (place it cs).map ([] :: ·)
  | (rep :: rest) :: cs =>
    if near rep it then some ((rep :: rest ++ [it]) :: cs)
    else (place it cs).map ((rep :: rest) :: ·)

def step (cl : List (List α)) (it : α) : List (List α) :=
  if zero it then cl ++ [[it]]
  else match place near it cl with
    | some cl' => cl'
    | none => cl ++ [[it]]

def clusterItems (items : List α) : List (List α) := items.foldl (step near zero) []

theorem place_perm (it : α) : ∀ (cl cl' : List (List α)), place near it cl = some cl' →
    cl'.flatten.Perm (cl.flatten ++ [it]) := by
  intro cl
  induction cl with
  | nil => intro cl' h; simp [place] at h
  | cons c cs ih =>
    intro cl' h
    cases c with
    | nil =>
      simp only [place, Option.map_eq_some_iff] at h
      obtain ⟨x, hx, rfl⟩ := h
      simpa using ih x hx
    | cons rep rest =>
      simp only [place] at h
      split at h
      · cases h
        simpa using List.Perm.append_left rest (List.perm_append_singleton it cs.flatten).symm
      · simp only [Option.map_eq_some_iff] at h
        obtain ⟨x, hx, rfl⟩ := h
        simp only [List.flatten_cons, List.append_assoc]
        exact List.Perm.append_left _ (ih x hx)

theorem place_nonempty (it : α) : ∀ (cl cl' : List (List α)), (∀ c ∈ cl, c ≠ []) →
    place near it cl = some cl' → ∀ c ∈ cl', c ≠ [] := by
  intro cl
  induction cl with
  | nil => intro cl' _ h; simp [place] at h
  | cons c cs ih =>
    intro cl' hne h
    cases c with
    | nil => exact absurd rfl (hne [] (by simp))
    | cons rep rest =>
      simp only [place] at h
      split at h
      · cases h; intro c hc
        simp only [List.mem_cons] at hc
        rcases hc with rfl | hc
        · simp
        · exact hne c (by simp [hc])
      · simp only [Option.map_eq_some_iff] at h
        obtain ⟨x, hx, rfl⟩ := h
        intro c hc
        simp only [List.mem_cons] at hc
        rcases hc with rfl | hc
        · simp
        · exact ih x (fun c h => hne c (by simp [h])) hx c hc

theorem foldl_perm : ∀ (items : List α) (acc : List (List α)),
    (items.foldl (step near zero) acc).flatten.Perm (acc.flatten ++ items) := by
  intro items
  induction items with
  | nil => intro acc; simp
  | cons it its ih =>
    intro acc
    simp only [List.foldl_cons]
    refine (ih _).trans ?_
    have hstep : (step near zero acc it).flatten.Perm (acc.flatten ++ [it]) := by
      unfold step
      split
      · simp
      · split
        · next cl' h => exact place_perm near it acc cl' h
        · simp
    have := hstep.append_right its
    simpa [List.append_assoc] using this

/-- No item is lost or duplicated: the clusters are a permutation of the input. -/
theorem clusterItems_perm (items : List α) :
    (clusterItems near zero items).flatten.Perm items := by
  unfold clusterItems; simpa using foldl_perm near zero items []

theorem foldl_nonempty : ∀ (items : List α) (acc : List (List α)), (∀ c ∈ acc, c ≠ []) →
    ∀ c ∈ items.foldl (step near zero) acc, c ≠ [] := by
  intro items
  induction items with
  | nil => intro acc h; simpa using h
  | cons it its ih =>
    intro acc hacc
    apply ih
    unfold step
    split
    · intro c hc; simp only [List.mem_append, List.mem_singleton] at hc
      rcases hc with hc | rfl
      · exact hacc c hc
      · simp
    · split
      · next cl' h => exact place_nonempty near it acc cl' hacc h
      · intro c hc; simp only [List.mem_append, List.mem_singleton] at hc
        rcases hc with hc | rfl
        · exact hacc c hc
        · simp

/-- Every cluster is non-empty, so `Cluster.Rep()` (Items[0]) never panics. -/
theorem clusterItems_nonempty (items : List α) :
    ∀ c ∈ clusterItems near zero items, c ≠ [] :=
  foldl_nonempty near zero items [] (by simp)

end Cluster

/-! ## 3. SimpleRateLimiter.WaitContext -/

/-- One WaitContext call at time `now`: returns (grant time, new lastCall).
    lastCall = now + max(minDelay - (now - lastCall), 0) = max(now, last + d). -/
def reserve (d last now : Int) : Int × Int :=
  let g := max now (last + d)
  (g, g)

/-- Successive reservations are at least `d` apart (the limiter's promise). -/
theorem reserve_spacing (d last now₁ now₂ : Int) :
    (reserve d (reserve d last now₁).2 now₂).1 ≥ (reserve d last now₁).1 + d := by
  simp only [reserve]; omega

/-- Calls in a trace; `true` = the caller's context was already cancelled.
    The code updates lastCall before it waits, in both cases. -/
def runTrace (d : Int) : Int → List (Int × Bool) → Int
  | last, [] => last
  | last, (now, _) :: rest => runTrace d (reserve d last now).2 rest

/-- Defect: ten cancelled callers at t=0 push the next real caller's grant to
    t = 11·d, although only one call was ever made. Expected: at most d. -/
theorem cancelled_waiters_delay_next_call :
    let d : Int := 50
    let last := runTrace d (-1000)
      ((0, false) :: List.replicate 10 ((0 : Int), true))
    (reserve d last 0).1 = 550 ∧ (reserve d last 0).1 > d := by
  decide
