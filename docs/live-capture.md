# Operating PortSmith on a live capture (sketch)

A proposal for discussion. The tooling is built around recorded captures; this is about the
step before that - turning a live stream into something the normalizer can read.

## The shape of the problem

Someone with a busy collector wants to point this at it, and today the answer is "capture
first, then run us". That is correct and stays correct, but the gap between the two steps
deserves to be written down.

## What a live mode would have to decide

| Question | Why it matters |
|---|---|
| Where is the boundary of a "record"? | The trace format is line-oriented; a live stream is not |
| What happens on a partial line at the tail? | It is the same problem `tail -f` has, and it is invisible until it corrupts a count |
| Does a session that never closes ever get inferred? | Collector inference with an open session is a guess, not a finding |
| What is the retention story? | A live mode that buffers without bound is a memory leak with a nicer name |

## Why this is a sketch and not a plan

Three of the four questions change the intermediate format if answered one way, and the format
is the contract both halves share. I would rather answer them with a capture in hand than
design it around an imagined one.
