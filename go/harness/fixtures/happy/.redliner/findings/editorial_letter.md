# Editorial Letter — The Gray Gull
### Developmental pass, Sections 1-2

## Summary

The opening has a real hook and a real craft problem sitting right next to it: Section 2's history dump stops the story cold at the exact moment it should be accelerating, and two POV leaks let Renne's deliberate opacity collapse before the uncertainty around her has a chance to build. Underneath both of those is a thinner issue — Mira's want is asserted rather than grounded, so the chase reads as an emergency happening to her rather than a choice she's making. None of this needs a rewrite. It needs relocation, a few cuts, and a handful of grounding lines.

## Your three questions, answered directly

### 1. Where should Chapter 2's history material go?

It doesn't have one destination — it has three, and they've been shoved into the same paragraph.

- **The uncle's-ledger-as-insurance motive is load-bearing.** It's the reason the ledger exists as an object worth stealing, and it needs to land early — worked into Mira's hiding beat in Section 1, or into the Renne exchange where payment and danger are already being negotiated.
- **The Voss family dynasty belongs at Voss's entrance**, not before it. Introducing the family before the man defeats the purpose of the reveal.
- **Selkirk's founding and the customs office's institutional history are the only genuinely optional part of the block.** Seed it in fragments across later chapters, or cut it outright — nothing here is load-bearing.

`[dev-002]`

The sentence that currently invites all three onto the page at once is worth pointing at directly: *"Before she could board, she needed to understand everything that had led her here."* That's the narrator's need to explain, handed to Mira as a want. She doesn't need to understand her own backstory in this moment — she needs to get on the boat. Cut the sentence (or replace it with an actual in-scene want, like needing Renne to move faster) and the dump loses its excuse to exist where it is. `[dev-003]`

### 2. Is Mira's motivation legible before the ledger's stakes are explained?

Not yet.

You've told the reader what Mira is doing — running, stealing, seeking passage — well before the reader knows why she wants it badly enough to risk this much. *"Mira had always known she would leave the harbor town of Selkirk"* asserts a longstanding want with no wound, no specific future, no personal cost underneath it. One concrete image or thought while she's hiding in the fish stall would ground it — this doesn't need to be exposition. `[dev-004]`

The stakes have the same problem from the other direction: the only stated consequence for the ledger falling into customs hands is collective — *"half the harbor would be in chains by morning."* Your own logline promises the uncle is a danger to Mira personally, but the prose never says what he, or customs, does to her specifically if she's caught. One clause would make the danger personal instead of civic. `[dev-005]`

### 3. Does the opening create enough forward pressure to carry a reader into Chapter 3?

Partially — and what pressure exists is leaking.

*"You're late"* implies a prearranged schedule the reader was never given, so there's no clock to feel ticking. Either seed the deadline earlier so the line pays off a promise already made, or cut it. Separately, the alarm bell and active pursuit from Section 1 evaporate with no bridge — Section 2 opens at a frictionless dawn boarding that doesn't acknowledge the chase was ever live. One beat connecting the two (a glance over her shoulder, a bell still audible) would keep the urgency continuous instead of resetting it. `[dev-006]`

Section 1's closing line is a genuine promise of complication: *"The ledger was worth more than Renne knew."* Section 2 doesn't cash it — Renne's payment demand gets deferred ("We'll talk about payment once we're clear of the harbor mouth") instead of used to extend that tension across the scene break. Use the boarding scene to actively spend the hook — a lie Mira has to tell, a question from Renne that shows she suspects more than she's saying. `[dev-007]`

## One more thing, at the same priority

This wasn't one of your three questions, but it sits at major severity and touches a choice you've explicitly protected: **Renne's opacity is leaking.** `[dev-001]`

Two lines hand the reader Renne's actual interior state instead of Mira's read of her from the outside. Lines 20-22 state Renne's private assessment as flat narration, entirely outside Mira's POV: *"Renne had seen a hundred desperate people come aboard... and this one would be no different."* Lines 28-30 are softer but still resolve what her expression means rather than leaving it unread. Neither is a case for developing Renne further — they're POV discipline slips that spend the uncertainty you're deliberately building before Act 1 has used it.

## Not addressed in this pass

Line-level editing is deferred until structure settles — see `line_notes` in the JSON companion to this letter. One item worth flagging now so it isn't lost: a continuity slip on Mira's eye color (green in Section 1, blue in Section 2) was caught independently in both the developmental pass and the continuity check — `[dev-009]` / `[cont-001]`. Same issue, two ids; pick one color and it's a line-pass fix, not a structural one.

## Priority order

1. `[dev-002]` — Relocate the Section 2 history block (three destinations, not one)
2. `[dev-003]` — Cut or replace the sentence anchoring that block
3. `[dev-001]` — Fix the two Renne POV leaks
4. `[dev-004]` — Ground Mira's want with one concrete beat
5. `[dev-006]` — Fix the unearned deadline and the missing chase-to-dock bridge
6. `[dev-007]` — Cash the "worth more than Renne knew" hook in the boarding scene
7. `[dev-005]` — Add a personal stake to the ledger's danger

## How to respond

- Work a finding: `/redliner:run work <id>`
- Mark one resolved once you've addressed it: `/redliner:run resolve <id>`
- Ask for a recheck after revisions: `/redliner:run recheck`
